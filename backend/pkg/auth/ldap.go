package auth

import (
	"crypto/tls"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

type LDAPConfig struct {
	Server   string `json:"server"`
	Port     int    `json:"port"`
	BaseDN   string `json:"base_dn"`
	BindDN   string `json:"bind_dn"`
	BindPass string `json:"bind_pass"`
	Filter   string `json:"filter"`
	UseTLS   bool   `json:"use_tls"`
	Timeout  int    `json:"timeout"`
}

type LDAPAuthenticator struct {
	config LDAPConfig
}

func NewLDAPAuthenticator(config LDAPConfig) *LDAPAuthenticator {
	if config.Timeout == 0 {
		config.Timeout = 10
	}
	if config.Port == 0 {
		config.Port = 389
	}
	if config.Filter == "" {
		config.Filter = "(uid=%s)"
	}
	return &LDAPAuthenticator{config: config}
}

func (a *LDAPAuthenticator) Authenticate(username, password string) (map[string]string, error) {
	addr := net.JoinHostPort(a.config.Server, strconv.Itoa(a.config.Port))
	conn, err := net.DialTimeout("tcp", addr, time.Duration(a.config.Timeout)*time.Second)
	if err != nil {
		return nil, fmt.Errorf("ldap connect failed: %w", err)
	}
	defer conn.Close()

	if a.config.UseTLS {
		tlsConn := tls.Client(conn, &tls.Config{InsecureSkipVerify: true})
		if err := tlsConn.Handshake(); err != nil {
			return nil, fmt.Errorf("ldap tls handshake failed: %w", err)
		}
		conn = tlsConn
	}

	ldapMsg := buildBindRequest(a.config.BindDN, a.config.BindPass)
	if _, err := conn.Write(ldapMsg); err != nil {
		return nil, fmt.Errorf("ldap bind write failed: %w", err)
	}

	resp := make([]byte, 4096)
	n, err := conn.Read(resp)
	if err != nil {
		return nil, fmt.Errorf("ldap bind read failed: %w", err)
	}
	if n < 2 || resp[1] != 0x00 {
		return nil, fmt.Errorf("ldap bind failed: server returned error code")
	}

	userDN := fmt.Sprintf(a.config.Filter, username)
	if !strings.Contains(a.config.Filter, "%s") {
		userDN = a.config.Filter
	}

	ldapMsg = buildBindRequest(userDN, password)
	if _, err := conn.Write(ldapMsg); err != nil {
		return nil, fmt.Errorf("ldap user bind failed: %w", err)
	}

	n, err = conn.Read(resp)
	if err != nil {
		return nil, fmt.Errorf("ldap user bind read failed: %w", err)
	}
	if n < 2 || resp[1] != 0x00 {
		return nil, fmt.Errorf("invalid ldap credentials")
	}

	return map[string]string{
		"username":   username,
		"dn":         userDN,
	}, nil
}

func buildBindRequest(dn, password string) []byte {
	// Minimal LDAP BIND request (simple auth)
	var req []byte
	req = append(req, 0x30)
	body := append([]byte{0x02, 0x01, 0x01}, // version 3
		append([]byte{0x60}, encodeLength(len(dn)+2)...)...) // bind request tag
	body = append(body, append([]byte{0x04}, encodeLength(len(dn))...)...)
	body = append(body, []byte(dn)...)
	body = append(body, 0x80)
	body = append(body, encodeLength(len(password))...)
	body = append(body, []byte(password)...)
	req = append(req, encodeLength(len(body))...)
	req = append(req, body...)
	return req
}

func encodeLength(n int) []byte {
	if n < 128 {
		return []byte{byte(n)}
	}
	return []byte{0x81, byte(n)}
}
