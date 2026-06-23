package notifier

import (
	"fmt"
	"sync"
)

var (
	mu        sync.RWMutex
	providers = map[string]Provider{
		"email":   &EmailProvider{},
		"dingtalk": NewDingTalkProvider(),
		"wecom":   NewWeComProvider(),
	}
)

func Register(name string, p Provider) {
	mu.Lock()
	providers[name] = p
	mu.Unlock()
}

func Get(name string) Provider {
	mu.RLock()
	defer mu.RUnlock()
	return providers[name]
}

func Send(channelType, channelConfig, title, content string) error {
	p := Get(channelType)
	if p == nil {
		return fmt.Errorf("unknown channel type: %s", channelType)
	}
	return p.Send(channelConfig, title, content)
}
