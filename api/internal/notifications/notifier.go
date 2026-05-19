package notifications

import (
	"context"
	"sync"
)

// Message is a single outbound notification.
type Message struct {
	GroupID  string
	To       string
	Subject  string
	Body     string // HTML
	TextBody string // plain text fallback

	// Threading fields — set by sendTo before calling Notifier.Send.
	// MessageID is the caller-generated Message-ID for this send (first in thread).
	// InReplyTo is the Message-ID of the first message in the thread (for follow-ups).
	// Both are empty strings when threading is not applicable.
	MessageID string
	InReplyTo string

	// ThreadKey is used by GChatNotifier to group messages into Chat API threads.
	// Not used by SMTPNotifier.
	ThreadKey string
	// ThreadName is the Chat API thread resource name (e.g. "spaces/X/threads/Y").
	// When set, it takes precedence over ThreadKey for replying to an existing thread.
	ThreadName string
}

// Notifier sends a notification message on a specific channel.
type Notifier interface {
	Send(ctx context.Context, msg Message) error
}

// PairedNotifier extends Notifier with a two-message threaded send for GChat broadcasts.
// SendPaired sends opener and detail as replies in the same thread, returning errors for each.
type PairedNotifier interface {
	Notifier
	SendPaired(ctx context.Context, groupID, space, opener, detail, threadKey string) (openerErr, detailErr error)
}

// NoopNotifier discards all messages. Used in production when SMTP is not configured.
type NoopNotifier struct{}

func (NoopNotifier) Send(_ context.Context, _ Message) error { return nil }

// CapturingNotifier records all messages sent. Used in tests.
type CapturingNotifier struct {
	mu   sync.Mutex
	Sent []Message
}

func (c *CapturingNotifier) Send(_ context.Context, msg Message) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Sent = append(c.Sent, msg)
	return nil
}

func (c *CapturingNotifier) Messages() []Message {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]Message, len(c.Sent))
	copy(out, c.Sent)
	return out
}

func (c *CapturingNotifier) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Sent = nil
}
