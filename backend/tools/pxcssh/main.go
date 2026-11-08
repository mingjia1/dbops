// pxcssh: one-off helper to run commands / upload files on dbops test hosts
// using SSH credentials stored in the platform DB.
//
// Usage:
//   go run ./tools/pxcssh -host 10.1.81.21 -cmd "hostname"
//   go run ./tools/pxcssh -host 10.1.81.21 -upload local=bin/agent-linux-amd64 remote=/tmp/agent-new
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"time"

	_ "modernc.org/sqlite"

	"golang.org/x/crypto/ssh"

	"github.com/jackcode/mysql-ops-platform/pkg/utils"
)

func main() {
	host := flag.String("host", "", "remote host address")
	port := flag.Int("port", 22, "ssh port")
	cmd := flag.String("cmd", "", "command to run")
	upLocal := flag.String("upload", "", "local file to upload")
	upRemote := flag.String("remote", "", "remote path for -upload")
	flag.Parse()

	if *host == "" {
		fmt.Fprintln(os.Stderr, "-host required")
		os.Exit(2)
	}

	user, password, err := credentialFor(*host)
	if err != nil {
		fmt.Fprintln(os.Stderr, "credential:", err)
		os.Exit(1)
	}

	client, err := dial(*host, *port, user, password)
	if err != nil {
		fmt.Fprintln(os.Stderr, "dial:", err)
		os.Exit(1)
	}
	defer client.Close()

	if *upLocal != "" {
		if err := upload(client, *upLocal, *upRemote); err != nil {
			fmt.Fprintln(os.Stderr, "upload:", err)
			os.Exit(1)
		}
		fmt.Printf("uploaded %s -> %s:%s\n", *upLocal, *host, *upRemote)
		return
	}

	if *cmd != "" {
		out, err := run(client, *cmd)
		fmt.Print(out)
		if err != nil {
			fmt.Fprintln(os.Stderr, "cmd exit:", err)
			os.Exit(1)
		}
		return
	}
	fmt.Fprintln(os.Stderr, "nothing to do: use -cmd or -upload")
	os.Exit(2)
}

// credentialFor loads ssh_user / ssh_credential straight from the platform
// sqlite DB and decrypts the password with DBOPS_ENCRYPTION_KEY.
func credentialFor(address string) (string, string, error) {
	dbPath := os.Getenv("DBOPS_DB_PATH")
	if dbPath == "" {
		dbPath = "../db/dbops.db"
	}
	key := os.Getenv("DBOPS_ENCRYPTION_KEY")
	if key == "" {
		return "", "", fmt.Errorf("DBOPS_ENCRYPTION_KEY not set")
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return "", "", err
	}
	defer db.Close()
	var user, cred string
	err = db.QueryRow("SELECT ssh_user, COALESCE(ssh_credential,'') FROM hosts WHERE address = ?", address).Scan(&user, &cred)
	if err != nil {
		return "", "", err
	}
	plain, derr := utils.Decrypt(cred, key)
	if derr != nil {
		return "", "", derr
	}
	return user, plain, nil
}

func dial(host string, port int, user, password string) (*ssh.Client, error) {
	cfg := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.Password(password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // ponytail: test env only
		Timeout:         10 * time.Second,
	}
	return ssh.Dial("tcp", net.JoinHostPort(host, fmt.Sprintf("%d", port)), cfg)
}

func run(client *ssh.Client, command string) (string, error) {
	sess, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer sess.Close()
	out, err := sess.CombinedOutput(command)
	return string(out), err
}

// upload streams the local file to the remote path via `cat`.
func upload(client *ssh.Client, localPath, remotePath string) error {
	f, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer f.Close()
	sess, err := client.NewSession()
	if err != nil {
		return err
	}
	defer sess.Close()
	sess.Stderr = os.Stderr
	stdin, err := sess.StdinPipe()
	if err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() {
		_, err := io.Copy(stdin, f)
		closeErr := stdin.Close()
		done <- firstErr(err, closeErr)
	}()
	err = sess.Run(fmt.Sprintf("cat > %s && chmod +x %s", remotePath, remotePath))
	if err != nil {
		return err
	}
	return <-done
}

func firstErr(errs ...error) error {
	for _, e := range errs {
		if e != nil {
			return e
		}
	}
	return nil
}
