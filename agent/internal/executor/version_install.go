package executor

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"time"
)

// InstallFromURL downloads a MySQL/MariaDB/Percona tarball to basedir, verifies
// its sha256, extracts it, and creates the OS user. This is the generic
// version-agnostic installer used by both the deploy and upgrade flows.
//
// Inputs:
//   - packageURL : "https://dev.mysql.com/.../mysql-8.0.36-linux-glibc2.17-x86_64.tar.xz"
//   - expectedSHA256: ""  (skip verify) or hex string of 64 chars
//   - basedir     : "/opt/mysql-8.0.36" (where to extract)
//   - osUser      : "mysql" (create if missing)
//
// On success, basedir contains:
//   - <basedir>/<flavor>-<version>-linux-glibc*-x86_64/bin/mysqld
//   - the OS user exists, with shell ownership of basedir.
//
// Returns the absolute path to the mysqld binary inside basedir.
func InstallFromURL(ctx context.Context, packageURL, expectedSHA256, basedir, osUser string) (string, error) {
	return InstallFromURLWithRelay(ctx, packageURL, expectedSHA256, basedir, osUser, "")
}

func InstallFromURLWithRelay(ctx context.Context, packageURL, expectedSHA256, basedir, osUser, relayUploadURL string) (string, error) {
	if packageURL == "" {
		return "", fmt.Errorf("package_url is required for version-agnostic install")
	}
	if basedir == "" {
		basedir = "/opt/mysql"
	}
	if osUser == "" {
		osUser = "mysql"
	}

	// 1. ensure OS user exists
	if err := ensureOSUser(osUser); err != nil {
		return "", fmt.Errorf("ensure user %s: %w", osUser, err)
	}

	// 2. download to /tmp
	tarballPath := filepath.Join("/tmp", filepath.Base(packageURL))
	if err := downloadFile(ctx, packageURL, tarballPath, expectedSHA256); err != nil {
		return "", fmt.Errorf("download: %w", err)
	}
	defer os.Remove(tarballPath)

	// 2b. upload to relay server if relay upload URL is provided
	// This caches the package on the relay for other hosts to use.
	if relayUploadURL != "" {
		go uploadToRelay(relayUploadURL, tarballPath)
	}

	// 3. extract to basedir
	if err := extractTarball(ctx, tarballPath, basedir); err != nil {
		return "", fmt.Errorf("extract: %w", err)
	}

	// 4. find mysqld binary
	mysqld := findMysqldBinary(basedir)
	if mysqld == "" {
		return "", fmt.Errorf("mysqld binary not found under %s after extract", basedir)
	}

	// 5. chown -R
	uid, gid := lookupUserIDs(osUser)
	if err := chownRecursive(basedir, uid, gid); err != nil {
		return "", fmt.Errorf("chown: %w", err)
	}

	// 6. run ldconfig to update shared library cache (handles libprotobuf, libssl, etc.)
	if err := runLdconfig(ctx, basedir); err != nil {
		return "", fmt.Errorf("ldconfig: %w", err)
	}

	return mysqld, nil
}

func ensureOSUser(username string) error {
	_, err := user.Lookup(username)
	if err == nil {
		return nil
	}
	// useradd -r -s /bin/false -d /var/lib/mysql <user>
	cmd := exec.Command("useradd", "-r", "-s", "/bin/false", "-d", "/var/lib/"+username, username)
	if out, err := cmd.CombinedOutput(); err != nil {
		// ignore "already exists" race
		if !strings.Contains(string(out), "exists") {
			return fmt.Errorf("useradd: %v: %s", err, string(out))
		}
	}
	return nil
}

func lookupUserIDs(username string) (int, int) {
	u, err := user.Lookup(username)
	if err != nil {
		return -1, -1
	}
	uid := atoiSafe(u.Uid)
	gid := atoiSafe(u.Gid)
	return uid, gid
}

func atoiSafe(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return -1
		}
		n = n*10 + int(c-'0')
	}
	return n
}

func chownRecursive(path string, uid, gid int) error {
	if uid < 0 || gid < 0 {
		// fall back to "mysql:mysql" by name
		cmd := exec.Command("chown", "-R", "mysql:mysql", path)
		return cmd.Run()
	}
	cmd := exec.Command("chown", "-R", fmt.Sprintf("%d:%d", uid, gid), path)
	return cmd.Run()
}

// downloadFile fetches URL to dest. If expectedSHA256 is non-empty, verifies.
func downloadFile(ctx context.Context, url, dest, expectedSHA256 string) error {
	c := &http.Client{Timeout: 60 * time.Minute}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := c.Do(req)
	if err != nil {
		return fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()

	hasher := sha256.New()
	w := io.MultiWriter(f, hasher)
	if _, err := io.Copy(w, resp.Body); err != nil {
		return err
	}
	if expectedSHA256 != "" {
		got := hex.EncodeToString(hasher.Sum(nil))
		if !strings.EqualFold(got, expectedSHA256) {
			return fmt.Errorf("sha256 mismatch: got %s want %s", got, expectedSHA256)
		}
	}
	return nil
}

// extractTarball extracts .tar.gz or .tar.xz to dest.
func extractTarball(ctx context.Context, tarball, dest string) error {
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	var cmd *exec.Cmd
	switch {
	case strings.HasSuffix(tarball, ".tar.xz") || strings.HasSuffix(tarball, ".txz"):
		cmd = exec.CommandContext(ctx, "tar", "-xJf", tarball, "-C", dest, "--strip-components=1")
	case strings.HasSuffix(tarball, ".tar.gz") || strings.HasSuffix(tarball, ".tgz"):
		cmd = exec.CommandContext(ctx, "tar", "-xzf", tarball, "-C", dest, "--strip-components=1")
	default:
		return fmt.Errorf("unsupported tarball extension: %s", tarball)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("tar failed: %v: %s", err, string(out))
	}
	return nil
}

// findMysqldBinary walks dest looking for bin/mysqld.
// runLdconfig 更新动态链接库缓存。如果 basedir 包含 lib/ 目录，先加入 ldconfig 缓存。
func runLdconfig(ctx context.Context, basedir string) error {
	// Write persistent ldconfig config for MySQL libraries
	confDir := "/etc/ld.so.conf.d"
	libDirs := []string{
		filepath.Join(basedir, "lib"),
		filepath.Join(basedir, "lib", "mysql"),
		filepath.Join(basedir, "lib64"),
		filepath.Join(basedir, "lib", "private"),
		filepath.Join(basedir, "lib", "mysql", "private"),
		filepath.Join(basedir, "lib64", "private"),
	}
	var existing string
	for _, d := range libDirs {
		if st, err := os.Stat(d); err == nil && st.IsDir() {
			existing += d + "\n"
		}
	}
	if existing == "" {
		return nil
	}
	confFile := filepath.Join(confDir, "mysql-custom-ldconfig.conf")
	_ = os.WriteFile(confFile, []byte(existing), 0644)
	cmd := exec.CommandContext(ctx, "ldconfig")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ldconfig failed: %v: %s", err, string(out))
	}
	return nil
}

func findMysqldBinary(dest string) string {
	candidate := filepath.Join(dest, "bin", "mysqld")
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	// fallback: walk
	filepath.Walk(dest, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && filepath.Base(path) == "mysqld" {
			candidate = path
			return filepath.SkipAll
		}
		return nil
	})
	return candidate
}

// uploadToRelay uploads a tarball file to the backend relay upload endpoint.
// Runs in a goroutine; errors are logged but don't block the install.
func uploadToRelay(uploadURL, filePath string) {
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "relay upload: open %s failed: %v\n", filePath, err)
		return
	}
	defer file.Close()

	body := &strings.Builder{}
	writer := multipart.NewWriter(body)

	// Determine sub-path from file name (e.g. mysql/8.0.36/)
	fileName := filepath.Base(filePath)
	subPath := ""
	if strings.HasPrefix(strings.ToLower(fileName), "mysql-") {
		// Extract version: mysql-8.0.36-linux-... -> 8.0.36
		parts := strings.SplitN(fileName, "-", 4)
		if len(parts) >= 3 {
			subPath = "mysql/" + parts[2]
		}
	} else if strings.HasPrefix(strings.ToLower(fileName), "mariadb-") {
		parts := strings.SplitN(fileName, "-", 3)
		if len(parts) >= 2 {
			subPath = "mariadb/" + parts[1]
		}
	}

	if subPath != "" {
		writer.WriteField("path", subPath)
	}

	part, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "relay upload: create form file failed: %v\n", err)
		return
	}
	if _, err := io.Copy(part, file); err != nil {
		fmt.Fprintf(os.Stderr, "relay upload: copy failed: %v\n", err)
		return
	}
	writer.Close()

	req, err := http.NewRequest("POST", uploadURL, strings.NewReader(body.String()))
	if err != nil {
		fmt.Fprintf(os.Stderr, "relay upload: create request failed: %v\n", err)
		return
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{Timeout: 30 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "relay upload: POST %s failed: %v\n", uploadURL, err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(os.Stderr, "relay upload: POST %s returned %d: %s\n", uploadURL, resp.StatusCode, string(respBody))
		return
	}
	fmt.Fprintf(os.Stderr, "relay upload: %s uploaded to %s\n", fileName, uploadURL)
}
