package notifier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type WeComProvider struct {
	client *http.Client
}

func NewWeComProvider() *WeComProvider {
	return &WeComProvider{client: &http.Client{Timeout: 10 * time.Second}}
}

type wecomMsg struct {
	MsgType string `json:"msgtype"`
	Text    struct {
		Content string `json:"content"`
	} `json:"text"`
}

func (p *WeComProvider) Send(webhookURL, title, content string) error {
	msg := wecomMsg{MsgType: "text"}
	msg.Text.Content = title + "\n" + content
	body, _ := json.Marshal(msg)

	resp, err := p.client.Post(webhookURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("wecom send failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("wecom returned status %d", resp.StatusCode)
	}
	return nil
}
