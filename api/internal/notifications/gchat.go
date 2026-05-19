package notifications

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"

	"github.com/malarscouterna/ms-utrustning/api/internal/crypto"
	"github.com/malarscouterna/ms-utrustning/api/internal/db"
)

// GChatNotifier sends broadcast card messages to Google Chat Spaces via the REST API.
// Sending uses chat.bot scope (service account direct). Listing spaces and managing
// memberships uses DWD (chat.spaces.readonly + chat.memberships) impersonating gchat_admin_email.
// To must be the space resource name, e.g. "spaces/AAAA123".
// LabelTeam prepends the team name to every opener message — useful in dev when the same
// space is linked to multiple teams. Off by default in production.
type GChatNotifier struct {
	Q         *db.Queries
	LabelTeam bool
}

type serviceAccountKey struct {
	Type        string `json:"type"`
	PrivateKey  string `json:"private_key"`
	ClientEmail string `json:"client_email"`
	TokenURI    string `json:"token_uri"`
}

// GChatSpace is a Google Chat Space returned by the spaces.list API.
type GChatSpace struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	SpaceType   string `json:"spaceType"`
	CanAutoAdd  bool   `json:"can_auto_add"`  // admin is a member → bot can be added automatically
	BotIsMember bool   `json:"bot_is_member"` // bot is already in the space → link directly
}

// gchatBotToken returns a token for the service account itself (chat.bot scope, no impersonation).
// Use this for posting messages to spaces where the bot is already a member.
func gchatBotToken(ctx context.Context, saJSON []byte) (string, error) {
	return gchatJWT(ctx, saJSON, "https://www.googleapis.com/auth/chat.bot", "")
}

// gchatAdminToken returns a DWD token impersonating adminEmail with user-level Chat scopes.
// Use this for listing spaces.
func gchatAdminToken(ctx context.Context, saJSON []byte, adminEmail string) (string, error) {
	return gchatJWT(ctx, saJSON,
		"https://www.googleapis.com/auth/chat.spaces.readonly",
		adminEmail)
}

// gchatUserMembershipToken returns a DWD token impersonating adminEmail with
// chat.memberships.app scope, allowing that user to add the bot to spaces they belong to.
func gchatUserMembershipToken(ctx context.Context, saJSON []byte, adminEmail string) (string, error) {
	return gchatJWT(ctx, saJSON, "https://www.googleapis.com/auth/chat.memberships.app", adminEmail)
}

func gchatJWT(ctx context.Context, saJSON []byte, scope, sub string) (string, error) {
	var key serviceAccountKey
	if err := json.Unmarshal(saJSON, &key); err != nil {
		return "", fmt.Errorf("gchat: parse service account: %w", err)
	}

	block, _ := pem.Decode([]byte(key.PrivateKey))
	if block == nil {
		return "", fmt.Errorf("gchat: invalid private key PEM")
	}
	raw, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("gchat: parse private key: %w", err)
	}
	rsaKey, ok := raw.(*rsa.PrivateKey)
	if !ok {
		return "", fmt.Errorf("gchat: expected RSA private key")
	}

	tokenURI := key.TokenURI
	if tokenURI == "" {
		tokenURI = "https://oauth2.googleapis.com/token"
	}

	now := time.Now()
	claims := jwt.MapClaims{
		"iss":   key.ClientEmail,
		"scope": scope,
		"aud":   tokenURI,
		"iat":   now.Unix(),
		"exp":   now.Add(time.Hour).Unix(),
	}
	if sub != "" {
		claims["sub"] = sub
	}

	t := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signed, err := t.SignedString(rsaKey)
	if err != nil {
		return "", fmt.Errorf("gchat: sign JWT: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURI,
		strings.NewReader(url.Values{
			"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"},
			"assertion":  {signed},
		}.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("gchat: token exchange: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("gchat: token exchange failed (%d): %s", resp.StatusCode, body)
	}

	var tr struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &tr); err != nil {
		return "", fmt.Errorf("gchat: decode token response: %w", err)
	}
	return tr.AccessToken, nil
}

func gchatPost(ctx context.Context, token, path string, body interface{}) error {
	_, err := gchatPostGetThread(ctx, token, path, body)
	return err
}

// gchatPostGetThread posts a message and returns the thread name from the response
// (e.g. "spaces/AAAA/threads/BBBB"). The thread name is needed to reliably reply to
// an existing thread — threadKey alone only works for creating threads.
func gchatPostGetThread(ctx context.Context, token, path string, body interface{}) (threadName string, err error) {
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://chat.googleapis.com/v1/"+path,
		strings.NewReader(string(bodyJSON)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("gchat: POST %s: %w", path, err)
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("gchat: POST %s failed (%d): %s", path, resp.StatusCode, rb)
	}
	var result struct {
		Thread struct {
			Name string `json:"name"`
		} `json:"thread"`
	}
	_ = json.Unmarshal(rb, &result)
	return result.Thread.Name, nil
}

// listSpacesWithToken fetches all SPACE-type spaces accessible via the given token,
// following nextPageToken until all pages are retrieved.
func listSpacesWithToken(ctx context.Context, token string) ([]GChatSpace, error) {
	var all []GChatSpace
	pageToken := ""
	for {
		u := "https://chat.googleapis.com/v1/spaces?pageSize=100"
		if pageToken != "" {
			u += "&pageToken=" + url.QueryEscape(pageToken)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("gchat: list spaces: %w", err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("gchat: list spaces failed (%d): %s", resp.StatusCode, body)
		}

		var result struct {
			Spaces        []GChatSpace `json:"spaces"`
			NextPageToken string       `json:"nextPageToken"`
		}
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, err
		}
		for _, s := range result.Spaces {
			if s.SpaceType == "SPACE" {
				all = append(all, s)
			}
		}
		if result.NextPageToken == "" {
			break
		}
		pageToken = result.NextPageToken
	}
	slog.Debug("gchat list spaces", "count", len(all))
	return all, nil
}

// ListGChatSpaces returns the union of spaces where the admin is a member (can_auto_add)
// and spaces where the bot is already a member (bot_is_member), filtered to SPACE type.
// Spaces in neither category are omitted — they are not actionable.
func ListGChatSpaces(ctx context.Context, saJSON []byte, adminEmail string) ([]GChatSpace, error) {
	adminToken, err := gchatAdminToken(ctx, saJSON, adminEmail)
	if err != nil {
		return nil, err
	}
	botToken, err := gchatBotToken(ctx, saJSON)
	if err != nil {
		return nil, err
	}

	adminSpaces, err := listSpacesWithToken(ctx, adminToken)
	if err != nil {
		return nil, err
	}
	// Bot space listing is best-effort — if the bot has no spaces yet, return an empty list.
	botSpaces, botErr := listSpacesWithToken(ctx, botToken)
	if botErr != nil {
		slog.Warn("gchat: bot space listing failed (bot_is_member will be derived from DB)", "err", botErr)
	}

	botSet := make(map[string]bool, len(botSpaces))
	for _, s := range botSpaces {
		botSet[s.Name] = true
	}

	// Build result: admin spaces with flags, then any bot-only spaces.
	seen := make(map[string]bool, len(adminSpaces))
	result := make([]GChatSpace, 0, len(adminSpaces))
	for _, s := range adminSpaces {
		s.CanAutoAdd = true
		s.BotIsMember = botSet[s.Name]
		result = append(result, s)
		seen[s.Name] = true
	}
	for _, s := range botSpaces {
		if !seen[s.Name] {
			s.BotIsMember = true
			result = append(result, s)
		}
	}
	return result, nil
}

// AddBotToSpace adds the app to the space and posts a welcome message that includes teamName.
// Uses chat.memberships.app scope via DWD impersonating adminEmail.
// Requires adminEmail to be a member of the space. Ignores 409 (already a member).
func AddBotToSpace(ctx context.Context, saJSON []byte, adminEmail, spaceName, teamName string) error {
	membershipToken, err := gchatUserMembershipToken(ctx, saJSON, adminEmail)
	if err != nil {
		return err
	}

	memberBody := map[string]interface{}{
		"member": map[string]interface{}{
			"name": "users/app",
			"type": "BOT",
		},
	}
	bodyJSON, _ := json.Marshal(memberBody)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://chat.googleapis.com/v1/"+spaceName+"/members",
		strings.NewReader(string(bodyJSON)))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+membershipToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("gchat: add bot member: %w", err)
	}
	rb, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != 409 {
		return fmt.Errorf("gchat: add bot member failed (%d): %s", resp.StatusCode, rb)
	}

	botToken, err := gchatBotToken(ctx, saJSON)
	if err != nil {
		return err
	}
	return gchatPost(ctx, botToken, spaceName+"/messages", map[string]interface{}{
		"text": fmt.Sprintf("✅ Google Chat kopplat för avdelningen *%s*. Boknings- och ärendenotiser kommer att skickas hit.", teamName),
	})
}

func (g *GChatNotifier) Send(ctx context.Context, msg Message) error {
	creds, err := g.Q.GetGchatCredentials(ctx, msg.GroupID)
	if err != nil || len(creds.GchatServiceAccountJsonEncrypted) == 0 {
		return fmt.Errorf("gchat: no credentials for group %s", msg.GroupID)
	}

	saJSON, err := crypto.Decrypt(creds.GchatServiceAccountJsonEncrypted)
	if err != nil {
		return fmt.Errorf("gchat: decrypt credentials: %w", err)
	}

	token, err := gchatBotToken(ctx, saJSON)
	if err != nil {
		return err
	}

	chatMsg := map[string]interface{}{
		"text": msg.Subject,
	}
	if msg.ThreadName != "" {
		// Reply to an existing thread by its API name — more reliable than threadKey.
		chatMsg["thread"] = map[string]interface{}{"name": msg.ThreadName}
	} else if msg.ThreadKey != "" {
		chatMsg["thread"] = map[string]interface{}{"threadKey": msg.ThreadKey}
	}

	// msg.To is the full space resource name, e.g. "spaces/AAAA123".
	_, err = gchatPostGetThread(ctx, token, msg.To+"/messages?messageReplyOption=REPLY_MESSAGE_FALLBACK_TO_NEW_THREAD", chatMsg)
	return err
}

// SendPaired sends opener and detail as two messages in the same thread.
// The opener creates (or joins) a thread via threadKey; the detail replies using the
// thread name returned by the API, which is more reliable than re-sending threadKey.
// If the opener send fails, the detail is skipped. If detail is empty, it is not sent.
func (g *GChatNotifier) SendPaired(ctx context.Context, groupID, space, opener, detail, threadKey string) (openerErr, detailErr error) {
	creds, err := g.Q.GetGchatCredentials(ctx, groupID)
	if err != nil || len(creds.GchatServiceAccountJsonEncrypted) == 0 {
		return fmt.Errorf("gchat: no credentials for group %s", groupID), nil
	}
	saJSON, err := crypto.Decrypt(creds.GchatServiceAccountJsonEncrypted)
	if err != nil {
		return fmt.Errorf("gchat: decrypt credentials: %w", err), nil
	}
	token, err := gchatBotToken(ctx, saJSON)
	if err != nil {
		return err, nil
	}

	openerMsg := map[string]interface{}{
		"text":   opener,
		"thread": map[string]interface{}{"threadKey": threadKey},
	}
	threadName, openerErr := gchatPostGetThread(ctx, token,
		space+"/messages?messageReplyOption=REPLY_MESSAGE_FALLBACK_TO_NEW_THREAD", openerMsg)
	if openerErr != nil || detail == "" {
		return openerErr, nil
	}

	detailThread := map[string]interface{}{"threadKey": threadKey}
	if threadName != "" {
		detailThread = map[string]interface{}{"name": threadName}
	}
	detailMsg := map[string]interface{}{
		"text":   detail,
		"thread": detailThread,
	}
	detailErr = gchatPost(ctx, token,
		space+"/messages?messageReplyOption=REPLY_MESSAGE_FALLBACK_TO_NEW_THREAD", detailMsg)
	return nil, detailErr
}
