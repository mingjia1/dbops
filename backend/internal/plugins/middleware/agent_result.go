package middleware

import (
	"fmt"
	"strings"
)

func ensureAgentTaskSucceeded(component, host string, resp map[string]interface{}) error {
	if resp == nil {
		return fmt.Errorf("%s setup on %s returned empty agent response", component, host)
	}
	rawStatus, ok := resp["status"]
	status := ""
	if ok && rawStatus != nil {
		status = strings.ToLower(strings.TrimSpace(fmt.Sprint(rawStatus)))
	}
	switch status {
	case "completed", "success", "succeeded", "ok":
		return nil
	case "":
		return fmt.Errorf("%s setup on %s returned missing agent status", component, host)
	default:
		message := strings.TrimSpace(fmt.Sprint(resp["message"]))
		if message == "" || message == "<nil>" {
			message = "no message"
		}
		return fmt.Errorf("%s setup on %s returned status %s: %s", component, host, status, message)
	}
}
