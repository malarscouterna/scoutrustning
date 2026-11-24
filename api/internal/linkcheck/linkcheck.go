// Package linkcheck verifies that article links still resolve. It is used by
// the daily background job and the "check now" action in group settings.
//
// Requests are made by the server on behalf of a manager, so they are treated
// as untrusted input: only http(s) is allowed and loopback/private/link-local
// destinations are refused (unless the test constructor is used), which keeps
// the checker from being turned into an SSRF probe.
package linkcheck

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/malarscouterna/scoutrustning/api/internal/db"
)

const (
	userAgent   = "ScoutrustningLinkChecker/1.0 (+https://github.com/malarscouterna/scoutrustning)"
	maxURLBytes = 2048
	// Only this much of a response body is read before closing it - enough for
	// servers that choke on HEAD but not worth downloading a whole PDF.
	bodyLimit = 64 * 1024
)

// Result is the outcome of checking one URL.
type Result struct {
	OK          bool
	HTTPStatus  int
	FailureKind string
	Error       string
}

// Checker checks URLs safely.
type Checker struct {
	client       *http.Client
	allowPrivate bool
}

// New returns a checker suitable for production: private and loopback targets
// are refused.
func New() *Checker { return newChecker(false) }

// NewForTest allows loopback targets so tests can point at an httptest server.
func NewForTest() *Checker { return newChecker(true) }

func newChecker(allowPrivate bool) *Checker {
	c := &Checker{allowPrivate: allowPrivate}
	dialer := &net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}
	c.client = &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			// No proxy: a private proxy address would bypass the destination
			// checks, and link checking should reflect what members reach.
			DialContext:           c.dialContext(dialer),
			TLSHandshakeTimeout:   5 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
			MaxIdleConns:          10,
			DisableKeepAlives:     true,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return errors.New("too many redirects")
			}
			return c.validateURL(req.URL)
		},
	}
	return c
}

// Check performs one request and classifies the outcome. A nil error from the
// HTTP client is not required for a usable result: failures are reported via
// Result.FailureKind.
func (c *Checker) Check(ctx context.Context, rawURL string) Result {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return Result{FailureKind: "invalid", Error: err.Error()}
	}
	if err := c.validateURL(u); err != nil {
		return Result{FailureKind: "invalid", Error: err.Error()}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return Result{FailureKind: "invalid", Error: err.Error()}
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/pdf;q=0.8,*/*;q=0.5")

	resp, err := c.client.Do(req)
	if err != nil {
		kind, msg := classify(err)
		return Result{FailureKind: kind, Error: msg}
	}
	defer resp.Body.Close()
	_, _ = io.CopyN(io.Discard, resp.Body, bodyLimit)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return Result{OK: true, HTTPStatus: resp.StatusCode}
	}
	return Result{
		HTTPStatus:  resp.StatusCode,
		FailureKind: fmt.Sprintf("http:%d", resp.StatusCode),
		Error:       resp.Status,
	}
}

func (c *Checker) validateURL(u *url.URL) error {
	if u == nil {
		return errors.New("missing url")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("unsupported scheme %q", u.Scheme)
	}
	if u.Host == "" {
		return errors.New("missing host")
	}
	if u.User != nil {
		return errors.New("credentials in url are not allowed")
	}
	if len(u.String()) > maxURLBytes {
		return errors.New("url too long")
	}
	// IP literals are checked before dialling; hostnames are checked at dial
	// time after DNS resolution, so this only guards the obvious cases.
	if ip := net.ParseIP(u.Hostname()); ip != nil && !c.allowPrivate && isDisallowedIP(ip) {
		return errors.New("destination not allowed")
	}
	return nil
}

func (c *Checker) dialContext(dialer *net.Dialer) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, err
		}
		for _, ip := range ips {
			if !c.allowPrivate && isDisallowedIP(ip.IP) {
				return nil, fmt.Errorf("blocked destination address %s", ip.IP)
			}
		}
		if len(ips) == 0 {
			return nil, fmt.Errorf("no addresses for %s", host)
		}
		return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
	}
}

func isDisallowedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}
	if ip.IsUnspecified() || ip.IsMulticast() || ip.IsInterfaceLocalMulticast() {
		return true
	}
	// IPv4 carrier-grade NAT 100.64.0.0/10.
	if v4 := ip.To4(); v4 != nil && v4[0] == 100 && v4[1] >= 64 && v4[1] <= 127 {
		return true
	}
	return false
}

func classify(err error) (string, string) {
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "timeout", "timed out"
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return "dns", dnsErr.Error()
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "blocked destination"):
		return "blocked", msg
	case strings.Contains(msg, "certificate"), strings.Contains(msg, "tls:"):
		return "tls", msg
	default:
		return "connection", msg
	}
}

// RunStats summarises one checker run.
type RunStats struct {
	Checked   int
	Broken    int
	Total     int
	Truncated bool
}

// RunDue checks links that have not been checked within interval, at most
// maxCount. It is meant to be called on startup and from a ticker.
func RunDue(ctx context.Context, q *db.Queries, c *Checker, interval time.Duration, maxCount int32) (RunStats, error) {
	links, err := q.ListArticleLinksDueForCheck(ctx, db.ListArticleLinksDueForCheckParams{
		CheckedBefore: pgTime(time.Now().Add(-interval)),
		MaxCount:      maxCount,
	})
	if err != nil {
		return RunStats{}, err
	}
	return run(ctx, q, c, links), nil
}

// RunGroup checks every link in a group, used by the "check now" action.
func RunGroup(ctx context.Context, q *db.Queries, c *Checker, groupID string, maxCount int32) (RunStats, error) {
	total, err := q.CountArticleLinksForGroup(ctx, groupID)
	if err != nil {
		return RunStats{}, err
	}
	links, err := q.ListArticleLinksForGroup(ctx, db.ListArticleLinksForGroupParams{
		GroupID:  groupID,
		MaxCount: maxCount,
	})
	if err != nil {
		return RunStats{}, err
	}
	stats := run(ctx, q, c, links)
	stats.Total = int(total)
	// Checked < Total when the run hit its deadline or more links exist than
	// the per-run cap; callers can tell the manager that not everything was
	// refreshed.
	stats.Truncated = stats.Checked < stats.Total
	return stats, nil
}

func run(ctx context.Context, q *db.Queries, c *Checker, links []db.ArticleLink) RunStats {
	const workers = 6
	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var stats RunStats

	for _, link := range links {
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		go func(l db.ArticleLink) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			checkCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
			defer cancel()
			res := c.Check(checkCtx, l.Url)

			// A canceled/timed-out run must not persist a result: the failure
			// would be "context canceled", not a property of the link. Only a
			// per-check deadline is a real timeout result.
			if ctx.Err() != nil {
				return
			}

			status := "broken"
			if res.OK {
				status = "ok"
			}
			var httpStatus pgtype.Int4
			if res.HTTPStatus > 0 {
				httpStatus = pgtype.Int4{Int32: int32(res.HTTPStatus), Valid: true}
			}
			updateErr := q.UpdateArticleLinkCheck(context.WithoutCancel(ctx), db.UpdateArticleLinkCheckParams{
				ID:             l.ID,
				LastStatus:     status,
				LastHttpStatus: httpStatus,
				LastError:      res.Error,
				FailureKind:    res.FailureKind,
			})

			mu.Lock()
			if updateErr == nil {
				stats.Checked++
				if !res.OK {
					stats.Broken++
				}
			}
			mu.Unlock()
		}(link)
	}
	wg.Wait()
	return stats
}

func pgTime(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}
