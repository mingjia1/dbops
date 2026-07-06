package controllers

import "testing"

func TestPasswordChangeAllowedPath(t *testing.T) {
	allowed := []string{
		"/api/v1/auth/change-password",
		"/api/v1/auth/me",
	}
	for _, path := range allowed {
		if !isPasswordChangeAllowedPath(path) {
			t.Fatalf("expected %s to be allowed during forced password change", path)
		}
	}

	blocked := []string{
		"/api/v1/deployments",
		"/api/v1/hosts",
		"/api/v1/auth/reset-all-passwords",
	}
	for _, path := range blocked {
		if isPasswordChangeAllowedPath(path) {
			t.Fatalf("expected %s to be blocked during forced password change", path)
		}
	}
}
