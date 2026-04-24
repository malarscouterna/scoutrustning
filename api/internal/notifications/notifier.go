package notifications

import (
	"context"
	"sync"
)

// Message is a single outbound notification.
type Message struct {
	GroupID string
	To      string
	Subject string
	Body    string // HTML
}

// Notifier sends a notification message on a specific channel.
type Notifier interface {
	Send(ctx context.Context, msg Message) error
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
