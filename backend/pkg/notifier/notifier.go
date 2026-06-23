package notifier

type Provider interface {
	Send(channelConfig string, title, content string) error
}
