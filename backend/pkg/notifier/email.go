package notifier

import (
	"fmt"
	"net/smtp"
	"strings"
)

type EmailProvider struct{}

func (p *EmailProvider) Send(config, title, content string) error {
	parts := strings.SplitN(config, "|", 4)
	if len(parts) < 4 {
		return fmt.Errorf("invalid email config: want smtp_host|smtp_port|from|to, got %q", config)
	}
	host, port, from, to := parts[0], parts[1], parts[2], parts[3]
	msg := []byte(fmt.Sprintf("Subject: %s\r\n\r\n%s", title, content))
	return smtp.SendMail(host+":"+port, nil, from, strings.Split(to, ","), msg)
}
