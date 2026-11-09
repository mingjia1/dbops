package auth

import (
	"crypto/tls"
	"encoding/pem"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func testLDAPTLSListener(t *testing.T) (string, int, string) {
	t.Helper()
	server := httptest.NewTLSServer(http.NotFoundHandler())
	defer server.Close()
	caFile := filepath.Join(t.TempDir(), "ca.pem")
	require.NoError(t, os.WriteFile(caFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}), 0o600))

	listener, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: server.TLS.Certificates})
	require.NoError(t, err)
	t.Cleanup(func() { listener.Close() })
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		buffer := make([]byte, 4096)
		for i := 0; i < 2; i++ {
			if _, err := conn.Read(buffer); err != nil {
				return
			}
			if _, err := conn.Write([]byte{0x30, 0x00}); err != nil {
				return
			}
		}
	}()
	host, portText, err := net.SplitHostPort(listener.Addr().String())
	require.NoError(t, err)
	port, err := strconv.Atoi(portText)
	require.NoError(t, err)
	return host, port, caFile
}

func TestLDAPTLSVerification(t *testing.T) {
	host, port, caFile := testLDAPTLSListener(t)
	config := LDAPConfig{Server: host, Port: port, UseTLS: true, Filter: "uid=%s"}
	_, err := NewLDAPAuthenticator(config).Authenticate("alice", "secret")
	require.Error(t, err, "an untrusted LDAP certificate must be rejected")

	host, port, caFile = testLDAPTLSListener(t)
	config.Server, config.Port, config.CACertFile = host, port, caFile
	_, err = NewLDAPAuthenticator(config).Authenticate("alice", "secret")
	require.NoError(t, err, "a certificate signed by the configured CA must be accepted")
}
