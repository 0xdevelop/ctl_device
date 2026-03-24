package notify

// Notifier sends notifications to configured channels.
type Notifier struct {
	channel string
	target  string
}

// NewNotifier creates a new Notifier.
func NewNotifier(channel, target string) *Notifier {
	return &Notifier{channel: channel, target: target}
}

// Send sends a notification message (stub).
func (n *Notifier) Send(message string) error {
	return nil
}
