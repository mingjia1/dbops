package notifier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type DingTalkProvider struct {
	client *http.Client
}

func NewDingTalkProvider() *DingTalkProvider {
	return &DingTalkProvider{client: &http.Client{Timeout: 10 * time.Second}}
}

type dingtalkMsg struct {
	MsgType string `json:"msgtype"`
	Text    struct {
		Content string `json:"content"`
	} `json:"text"`
}

func (p *DingTalkProvider) Send(webhookURL, title, content string) error {
	msg := dingtalkMsg{MsgType: "text"}
	msg.Text.Content = title + "\n" + content
	body, _ := json.Marshal(msg)

	resp, err := p.client.Post(webhookURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("dingtalk send failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("dingtalk returned status %d", resp.StatusCode)
	}
	return nil
}
