package license

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

type LicenseData struct {
	LicenseKey string    `json:"license_key"`
	Tier       Tier      `json:"tier"`
	IssuedTo   string    `json:"issued_to"`
	IssuedAt   time.Time `json:"issued_at"`
	ExpiresAt  time.Time `json:"expires_at"`
	MaxNodes   int       `json:"max_nodes"`
	Signature  string    `json:"signature"`
}

func (l *LicenseData) IsExpired() bool {
	return time.Now().After(l.ExpiresAt)
}

func (l *LicenseData) IsValid() bool {
	return !l.IsExpired() && l.Tier != "" && l.Signature != ""
}

func (l *LicenseData) Features() []Feature {
	return FeaturesForTier(l.Tier)
}

func (l *LicenseData) HasFeature(f Feature) bool {
	return HasFeature(l.Tier, f)
}

func (l *LicenseData) payload() string {
	return fmt.Sprintf("%s|%s|%s|%s|%s|%d", l.LicenseKey, l.Tier, l.IssuedTo, l.IssuedAt.Format(time.RFC3339), l.ExpiresAt.Format(time.RFC3339), l.MaxNodes)
}

func (l *LicenseData) Sign(secret string) {
	l.Signature = signPayload(l.payload(), secret)
}

func (l *LicenseData) Verify(secret string) bool {
	if l.Signature == "" {
		return false
	}
	expected := signPayload(l.payload(), secret)
	return hmac.Equal([]byte(l.Signature), []byte(expected))
}

func signPayload(payload, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

func Generate(tier Tier, issuedTo string, duration time.Duration, maxNodes int, secret string) *LicenseData {
	now := time.Now()
	l := &LicenseData{
		LicenseKey: fmt.Sprintf("DBOPS-%s-%d", tier, now.Unix()),
		Tier:       tier,
		IssuedTo:   issuedTo,
		IssuedAt:   now,
		ExpiresAt:  now.Add(duration),
		MaxNodes:   maxNodes,
	}
	l.Sign(secret)
	return l
}

func Parse(jsonData []byte, secret string) (*LicenseData, error) {
	var l LicenseData
	if err := json.Unmarshal(jsonData, &l); err != nil {
		return nil, fmt.Errorf("invalid license data: %w", err)
	}
	if !l.Verify(secret) {
		return nil, fmt.Errorf("license signature verification failed")
	}
	return &l, nil
}

func MustParse(jsonData []byte, secret string) *LicenseData {
	l, err := Parse(jsonData, secret)
	if err != nil {
		panic(err)
	}
	return l
}
