package executor

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// ---------------------------------------------------------------------------
// RelayPackageManager —— 中继服务器包管理器
//
// 部署模型:
//   - 一台或多台机器上的 agent 开启 relay.enabled=true 充当包中继
//   - 部署目标主机上的 agent 配置 relay.relay_host 指向中继服务器
//   - 目标主机请求安装工具时，agent 优先从中继服务器获取安装包
//   - 中继服务器按需缓存: 本地有则直接返回，没有则从官方源下载后缓存再返回
//
// 包类型:
//   - mysql_apt_config  : mysql-apt-config.deb (MySQL APT 仓库配置包)
//   - percona_release   : percona-release.deb (Percona 仓库配置包)
//   - mysql_server_deb  : mysql-server/x.x.x-ubuntu.deb
//   - xtrabackup_deb    : percona-xtrabackup-xx.deb
//   - mysql_yum_rpm     : mysql-community-server.rpm
//   - xtrabackup_yum_rpm : percona-xtrabackup-xx.rpm
// ---------------------------------------------------------------------------

type RelayPackageManager struct {
	cacheDir        string
	maxCacheSizeGB  int
	cacheExpireHours int
	mu              sync.RWMutex
	index           *RelayPackageIndex
	indexPath       string
}

type RelayPackageIndex struct {
	Packages  []RelayPackageMeta `json:"packages"`
	UpdatedAt time.Time          `json:"updated_at"`
}

type RelayPackageMeta struct {
	Name          string    `json:"name"`
	FileName      string    `json:"file_name"`
	Type          string    `json:"type"`
	Version       string    `json:"version"`
	OS            string    `json:"os"`
	Arch          string    `json:"arch"`
	Distro        string    `json:"distro"`
	Codename      string    `json:"codename,omitempty"`
	SourceURL     string    `json:"source_url"`
	SizeBytes     int64     `json:"size_bytes"`
	SHA256        string    `json:"sha256"`
	CachedAt      time.Time `json:"cached_at"`
	AccessCount   int       `json:"access_count"`
	LastAccessed  time.Time `json:"last_accessed"`
}

type RelayPackageRequest struct {
	URL            string `json:"url"`
	Name           string `json:"name"`
	Type           string `json:"type"`
	Version        string `json:"version"`
	ExpectedSHA256 string `json:"expected_sha256"`
	ForceRefresh   bool   `json:"force_refresh"`
}

type RelayPackageResult struct {
	Status    string `json:"status"`
	Message   string `json:"message"`
	FileName  string `json:"file_name"`
	FileSize  int64  `json:"file_size"`
	SHA256    string `json:"sha256"`
	FromCache bool   `json:"from_cache"`
}

type RelayStatus struct {
	Enabled      bool               `json:"enabled"`
	CacheDir     string             `json:"cache_dir"`
	PackageCount int                `json:"package_count"`
	TotalSizeMB  int64              `json:"total_size_mb"`
	MaxSizeGB    int                `json:"max_size_gb"`
	Packages     []RelayPackageMeta `json:"packages"`
}

type RelayPrefetchRequest struct {
	MySQLVersion string `json:"mysql_version"`
	Distro       string `json:"distro"`
	Codename     string `json:"codename,omitempty"`
}

func NewRelayPackageManager(cacheDir string, maxCacheSizeGB, cacheExpireHours int) *RelayPackageManager {
	if cacheDir == "" {
		cacheDir = "/data/relay/packages"
	}
	if maxCacheSizeGB <= 0 {
		maxCacheSizeGB = 50
	}
	if cacheExpireHours <= 0 {
		cacheExpireHours = 168 // 7 days
	}

	rpm := &RelayPackageManager{
		cacheDir:         cacheDir,
		maxCacheSizeGB:   maxCacheSizeGB,
		cacheExpireHours: cacheExpireHours,
		indexPath:        filepath.Join(cacheDir, "index.json"),
	}

	_ = os.MkdirAll(cacheDir, 0755)
	rpm.loadIndex()
	rpm.cleanupExpired()

	return rpm
}

func (r *RelayPackageManager) loadIndex() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.index = &RelayPackageIndex{
		Packages:  make([]RelayPackageMeta, 0),
		UpdatedAt: time.Now(),
	}

	data, err := os.ReadFile(r.indexPath)
	if err != nil {
		return
	}

	var idx RelayPackageIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return
	}
	r.index = &idx

	var validPkgs []RelayPackageMeta
	for _, pkg := range r.index.Packages {
		pkgPath := r.packagePath(pkg.FileName)
		if _, err := os.Stat(pkgPath); err == nil {
			validPkgs = append(validPkgs, pkg)
		}
	}
	r.index.Packages = validPkgs
}

func (r *RelayPackageManager) saveIndex() {
	r.mu.RLock()
	idx := *r.index
	r.mu.RUnlock()

	idx.UpdatedAt = time.Now()
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(r.indexPath, data, 0644)
}

func (r *RelayPackageManager) packagePath(fileName string) string {
	safe := filepath.Base(fileName)
	return filepath.Join(r.cacheDir, safe)
}

func (r *RelayPackageManager) findCached(nameOrURL string) *RelayPackageMeta {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for i := range r.index.Packages {
		pkg := &r.index.Packages[i]
		if pkg.Name == nameOrURL || pkg.FileName == nameOrURL || pkg.SourceURL == nameOrURL {
			pkgPath := r.packagePath(pkg.FileName)
			if _, err := os.Stat(pkgPath); err == nil {
				return pkg
			}
		}
	}
	return nil
}

func (r *RelayPackageManager) findCachedByVersionAndType(version, pkgType string) *RelayPackageMeta {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for i := range r.index.Packages {
		pkg := &r.index.Packages[i]
		if pkg.Version == version && pkg.Type == pkgType {
			pkgPath := r.packagePath(pkg.FileName)
			if _, err := os.Stat(pkgPath); err == nil {
				return pkg
			}
		}
	}
	return nil
}

func (r *RelayPackageManager) GetStatus() *RelayStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var totalSize int64
	pkgs := make([]RelayPackageMeta, len(r.index.Packages))
	copy(pkgs, r.index.Packages)
	for _, pkg := range pkgs {
		totalSize += pkg.SizeBytes
	}

	return &RelayStatus{
		Enabled:      true,
		CacheDir:     r.cacheDir,
		PackageCount: len(pkgs),
		TotalSizeMB:  totalSize / 1024 / 1024,
		MaxSizeGB:    r.maxCacheSizeGB,
		Packages:     pkgs,
	}
}

func (r *RelayPackageManager) ListPackages() []RelayPackageMeta {
	r.mu.RLock()
	defer r.mu.RUnlock()

	pkgs := make([]RelayPackageMeta, len(r.index.Packages))
	copy(pkgs, r.index.Packages)
	return pkgs
}

func (r *RelayPackageManager) GetPackagePath(fileName string) (string, error) {
	r.mu.RLock()
	pkgPath := r.packagePath(fileName)
	if _, err := os.Stat(pkgPath); err != nil {
		r.mu.RUnlock()
		return "", fmt.Errorf("package %s not found in cache", fileName)
	}

	found := false
	for i := range r.index.Packages {
		if r.index.Packages[i].FileName == fileName {
			r.index.Packages[i].AccessCount++
			r.index.Packages[i].LastAccessed = time.Now()
			found = true
			break
		}
	}
	r.mu.RUnlock()

	if found {
		r.saveIndex()
	}
	return pkgPath, nil
}

func (r *RelayPackageManager) FetchAndCache(ctx context.Context, req RelayPackageRequest) (*RelayPackageResult, error) {
	if req.URL == "" {
		return nil, fmt.Errorf("source URL is required for relay package fetch")
	}

	if !req.ForceRefresh {
		if cached := r.findCached(req.URL); cached != nil {
			r.recordAccess(cached.FileName)
			return &RelayPackageResult{
				Status:    "success",
				Message:   "package served from relay cache",
				FileName:  cached.FileName,
				FileSize:  cached.SizeBytes,
				SHA256:    cached.SHA256,
				FromCache: true,
			}, nil
		}
		if req.Name != "" {
			if cached := r.findCached(req.Name); cached != nil {
				r.recordAccess(cached.FileName)
				return &RelayPackageResult{
					Status:    "success",
					Message:   "package served from relay cache (matched by name)",
					FileName:  cached.FileName,
					FileSize:  cached.SizeBytes,
					SHA256:    cached.SHA256,
					FromCache: true,
				}, nil
			}
		}
	}

	if err := r.ensureCacheSpace(500 * 1024 * 1024); err != nil {
		return nil, fmt.Errorf("insufficient cache space: %w", err)
	}

	fileName := req.Name
	if fileName == "" {
		fileName = filepath.Base(req.URL)
	}
	if fileName == "" || fileName == "." || fileName == "/" {
		fileName = fmt.Sprintf("relay-pkg-%d", time.Now().UnixNano())
	}
	filePath := r.packagePath(fileName)

	_ = os.Remove(filePath)
	_ = os.Remove(filePath + ".tmp")

	tmpPath := filePath + ".tmp"
	defer os.Remove(tmpPath)

	downloadCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	if err := r.downloadFile(downloadCtx, req.URL, tmpPath); err != nil {
		return nil, fmt.Errorf("download from source failed: %w", err)
	}

	sha256Hash, err := r.computeSHA256(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("compute SHA256 failed: %w", err)
	}

	if req.ExpectedSHA256 != "" && !strings.EqualFold(sha256Hash, req.ExpectedSHA256) {
		return nil, fmt.Errorf("SHA256 mismatch: got %s, expected %s", sha256Hash, req.ExpectedSHA256)
	}

	st, err := os.Stat(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("stat downloaded file: %w", err)
	}
	fileSize := st.Size()

	if err := os.Rename(tmpPath, filePath); err != nil {
		return nil, fmt.Errorf("move to cache failed: %w", err)
	}

	meta := RelayPackageMeta{
		Name:         req.Name,
		FileName:     fileName,
		Type:         req.Type,
		Version:      req.Version,
		OS:           "linux",
		Arch:         "x86_64",
		SourceURL:    req.URL,
		SizeBytes:    fileSize,
		SHA256:        sha256Hash,
		CachedAt:     time.Now(),
		AccessCount:  1,
		LastAccessed: time.Now(),
	}
	if req.Type == "" {
		meta.Type = r.classifyPackageType(fileName, req.URL)
	}
	if req.Version == "" {
		meta.Version = r.extractVersion(fileName)
	}

	r.addToIndex(meta)

	return &RelayPackageResult{
		Status:    "success",
		Message:   "package downloaded and cached on relay",
		FileName:  fileName,
		FileSize:  fileSize,
		SHA256:    sha256Hash,
		FromCache: false,
	}, nil
}

func (r *RelayPackageManager) recordAccess(fileName string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.index.Packages {
		if r.index.Packages[i].FileName == fileName {
			r.index.Packages[i].AccessCount++
			r.index.Packages[i].LastAccessed = time.Now()
			break
		}
	}
}

func (r *RelayPackageManager) downloadFile(ctx context.Context, url, dest string) error {
	client := &http.Client{Timeout: 10 * time.Minute}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
		return fmt.Errorf("write file failed: %w", err)
	}
	return nil
}

func (r *RelayPackageManager) computeSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func (r *RelayPackageManager) addToIndex(meta RelayPackageMeta) {
	r.mu.Lock()
	found := -1
	for i := range r.index.Packages {
		if r.index.Packages[i].FileName == meta.FileName {
			found = i
			break
		}
	}
	if found >= 0 {
		r.index.Packages[found] = meta
	} else {
		r.index.Packages = append(r.index.Packages, meta)
	}
	r.mu.Unlock()
	r.saveIndex()
}

func (r *RelayPackageManager) classifyPackageType(fileName, url string) string {
	lower := strings.ToLower(fileName) + strings.ToLower(url)
	switch {
	case strings.Contains(lower, "mysql-apt-config"):
		return "mysql_apt_config"
	case strings.Contains(lower, "mysql-community-server") || strings.Contains(lower, "mysql-server-"):
		return "mysql_server_deb"
	case strings.Contains(lower, "mysql-community-client") || strings.Contains(lower, "mysql-client-"):
		return "mysql_client_deb"
	case strings.Contains(lower, "percona-xtrabackup-24"):
		return "xtrabackup_24_deb"
	case strings.Contains(lower, "percona-xtrabackup-80"):
		return "xtrabackup_80_deb"
	case strings.Contains(lower, "percona-xtrabackup-84"):
		return "xtrabackup_84_deb"
	case strings.Contains(lower, "percona-release"):
		return "percona_release"
	case strings.Contains(lower, "mysql") && strings.Contains(lower, ".tar"):
		return "mysql_tarball"
	case strings.Contains(lower, "xtrabackup") && strings.Contains(lower, ".tar"):
		return "xtrabackup_tarball"
	case strings.Contains(lower, ".deb"):
		return "deb_package"
	case strings.Contains(lower, ".rpm"):
		return "rpm_package"
	default:
		return "generic"
	}
}

func (r *RelayPackageManager) extractVersion(fileName string) string {
	lower := strings.ToLower(fileName)
	if strings.Contains(lower, "5.7") {
		return "5.7"
	}
	if strings.Contains(lower, "8.0") {
		return "8.0"
	}
	if strings.Contains(lower, "8.4") {
		return "8.4"
	}
	parts := strings.Split(fileName, "-")
	for i, part := range parts {
		if len(part) > 0 && part[0] >= '0' && part[0] <= '9' && strings.Count(part, ".") >= 1 {
			return part
		}
		_ = i
	}
	return "unknown"
}

func (r *RelayPackageManager) ensureCacheSpace(requiredBytes int64) error {
	r.mu.RLock()
	var totalSize int64
	for _, pkg := range r.index.Packages {
		totalSize += pkg.SizeBytes
	}
	r.mu.RUnlock()

	maxBytes := int64(r.maxCacheSizeGB) * 1024 * 1024 * 1024
	available := maxBytes - totalSize

	if available >= requiredBytes {
		return nil
	}

	r.cleanupExpired()

	r.mu.RLock()
	totalSize = 0
	for _, pkg := range r.index.Packages {
		totalSize += pkg.SizeBytes
	}
	r.mu.RUnlock()
	available = maxBytes - totalSize
	if available >= requiredBytes {
		return nil
	}

	r.evictLeastUsed(requiredBytes - available)

	return nil
}

func (r *RelayPackageManager) evictLeastUsed(needBytes int64) {
	r.mu.Lock()
	if len(r.index.Packages) == 0 {
		r.mu.Unlock()
		return
	}

	sort.Slice(r.index.Packages, func(i, j int) bool {
		pi, pj := r.index.Packages[i], r.index.Packages[j]
		if pi.AccessCount != pj.AccessCount {
			return pi.AccessCount < pj.AccessCount
		}
		return pi.LastAccessed.Before(pj.LastAccessed)
	})

	var evicted int64
	var survivors []RelayPackageMeta
	for _, pkg := range r.index.Packages {
		if evicted < needBytes {
			pkgPath := r.packagePath(pkg.FileName)
			_ = os.Remove(pkgPath)
			evicted += pkg.SizeBytes
			continue
		}
		survivors = append(survivors, pkg)
	}
	r.index.Packages = survivors
	r.mu.Unlock()
	r.saveIndex()
}

func (r *RelayPackageManager) cleanupExpired() {
	r.mu.Lock()
	cutoff := time.Now().Add(-time.Duration(r.cacheExpireHours) * time.Hour)
	var valid []RelayPackageMeta
	for _, pkg := range r.index.Packages {
		if pkg.CachedAt.After(cutoff) || pkg.AccessCount > 0 {
			valid = append(valid, pkg)
		} else {
			pkgPath := r.packagePath(pkg.FileName)
			_ = os.Remove(pkgPath)
		}
	}
	r.index.Packages = valid
	r.mu.Unlock()
	r.saveIndex()
}

func (r *RelayPackageManager) RemovePackage(fileName string) error {
	r.mu.Lock()
	pkgPath := r.packagePath(fileName)
	_ = os.Remove(pkgPath)

	var remaining []RelayPackageMeta
	for _, pkg := range r.index.Packages {
		if pkg.FileName != fileName {
			remaining = append(remaining, pkg)
		}
	}
	r.index.Packages = remaining
	r.mu.Unlock()
	r.saveIndex()
	return nil
}

func (r *RelayPackageManager) PrefetchPackages(ctx context.Context, req RelayPrefetchRequest) ([]RelayPackageResult, error) {
	if req.MySQLVersion == "" {
		req.MySQLVersion = "8.0"
	}
	if req.Distro == "" {
		req.Distro = "ubuntu"
	}
	if req.Codename == "" {
		req.Codename = "jammy"
	}

	info := HostOSInfo{
		OS:       "linux",
		Arch:     "x86_64",
		Distribution:   req.Distro,
		Codename: req.Codename,
	}

	ti := NewToolInstaller()
	urls := ti.resolvePackageURLsForVersion(req.MySQLVersion, info)

	var results []RelayPackageResult
	for _, urlReq := range urls {
		fetchReq := RelayPackageRequest{
			URL:            urlReq.URL,
			Name:           urlReq.Name,
			Type:           urlReq.Type,
			Version:        req.MySQLVersion,
			ExpectedSHA256: "",
		}
		res, err := r.FetchAndCache(ctx, fetchReq)
		if err != nil {
			results = append(results, RelayPackageResult{
				Status:  "failed",
				Message: fmt.Sprintf("failed to prefetch %s: %v", urlReq.Name, err),
			})
			continue
		}
		results = append(results, *res)
	}

	return results, nil
}

// ---------------------------------------------------------------------------
// ToolInstaller 扩展 —— 包URL解析，供中继预取和客户端下载使用
// ---------------------------------------------------------------------------

type packageURLSpec struct {
	URL  string
	Name string
	Type string
}

func (t *ToolInstaller) resolvePackageURLsForVersion(mysqlVersion string, info HostOSInfo) []packageURLSpec {
	var urls []packageURLSpec
	codename := info.Codename
	if codename == "" {
		codename = "jammy"
	}

	switch info.Distribution {
	case "ubuntu", "debian":
		urls = append(urls, packageURLSpec{
			Name: fmt.Sprintf("mysql-apt-config_0.8.33-1_all.deb"),
			URL:  "https://dev.mysql.com/get/mysql-apt-config_0.8.33-1_all.deb",
			Type: "mysql_apt_config",
		})

		urls = append(urls, packageURLSpec{
			Name: fmt.Sprintf("percona-release_latest.%s_all.deb", codename),
			URL:  fmt.Sprintf("https://repo.percona.com/apt/percona-release_latest.%s_all.deb", codename),
			Type: "percona_release",
		})

		urls = append(urls, packageURLSpec{
			Name: fmt.Sprintf("mysql-server-%s-ubuntu.deb", mysqlVersion),
			URL:  fmt.Sprintf("https://repo.mysql.com/apt/ubuntu/pool/mysql-%s/m/mysql-community/%s", mysqlVersion, mysqlVersion),
			Type: "mysql_server_deb",
		})

		xtraVersion := t.resolveXtraBackupVersion(mysqlVersion)
		urls = append(urls, packageURLSpec{
			Name: fmt.Sprintf("percona-xtrabackup-%s-%s-ubuntu.deb", xtraVersion, mysqlVersion),
			URL:  fmt.Sprintf("https://repo.percona.com/apt/ubuntu/pool/main/p/percona-xtrabackup-%s/", xtraVersion),
			Type: fmt.Sprintf("xtrabackup_%s_deb", xtraVersion),
		})

	case "centos", "rhel", "rocky", "fedora", "almalinux", "oracle", "amzn":
		repoRPM := "https://dev.mysql.com/get/mysql80-community-release-el7-7.noarch.rpm"
		if strings.HasPrefix(mysqlVersion, "5.7") {
			repoRPM = "https://dev.mysql.com/get/mysql57-community-release-el7-11.noarch.rpm"
		} else if strings.HasPrefix(mysqlVersion, "8.0") {
			repoRPM = "https://dev.mysql.com/get/mysql80-community-release-el7-7.noarch.rpm"
		} else if strings.HasPrefix(mysqlVersion, "8.4") {
			repoRPM = "https://dev.mysql.com/get/mysql84-community-release-el7-1.noarch.rpm"
		}

		urls = append(urls, packageURLSpec{
			Name: "mysql-community-release.rpm",
			URL:  repoRPM,
			Type: "mysql_yum_repo",
		})

		urls = append(urls, packageURLSpec{
			Name: "percona-release-latest.noarch.rpm",
			URL:  "https://repo.percona.com/yum/percona-release-latest.noarch.rpm",
			Type: "percona_yum_repo",
		})

		urls = append(urls, packageURLSpec{
			Name: fmt.Sprintf("mysql-community-server-%s.rpm", mysqlVersion),
			URL:  fmt.Sprintf("https://repo.mysql.com/yum/mysql-%s-community/el/7/x86_64/", mysqlVersion),
			Type: "mysql_server_rpm",
		})

		xtraVersion := t.resolveXtraBackupVersion(mysqlVersion)
		urls = append(urls, packageURLSpec{
			Name: fmt.Sprintf("percona-xtrabackup-%s.rpm", xtraVersion),
			URL:  fmt.Sprintf("https://repo.percona.com/yum/release/7/RPMS/x86_64/"),
			Type: fmt.Sprintf("xtrabackup_%s_rpm", xtraVersion),
		})
	}

	return urls
}

// ---------------------------------------------------------------------------
// 中继客户端: 供 ToolInstaller 在安装工具时使用
// ---------------------------------------------------------------------------

type relayClient struct {
	host       string
	port       int
	token      string
	httpClient *http.Client
}

func newRelayClient(host string, port int, token string) *relayClient {
	return &relayClient{
		host:       host,
		port:       port,
		token:      token,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (rc *relayClient) fetchPackage(ctx context.Context, req RelayPackageRequest) (*RelayPackageResult, error) {
	url := fmt.Sprintf("http://%s:%d/agent/relay/fetch", rc.host, rc.port)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if rc.token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+rc.token)
	}

	resp, err := rc.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("relay fetch request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read relay response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("relay returned HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var wrapper struct {
		Code    int                 `json:"code"`
		Message string              `json:"message"`
		Data    *RelayPackageResult `json:"data"`
	}
	if err := json.Unmarshal(respBody, &wrapper); err != nil {
		return nil, fmt.Errorf("parse relay response: %w", err)
	}
	if wrapper.Data == nil {
		return nil, fmt.Errorf("relay response has no data: %s", wrapper.Message)
	}
	return wrapper.Data, nil
}

func (rc *relayClient) downloadPackage(ctx context.Context, fileName, dest string) error {
	url := fmt.Sprintf("http://%s:%d/agent/relay/packages/download?name=%s", rc.host, rc.port, fileName)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	if rc.token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+rc.token)
	}

	resp, err := rc.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("relay download request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("relay download HTTP %d: %s", resp.StatusCode, string(body))
	}

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
		return fmt.Errorf("write downloaded package: %w", err)
	}
	return nil
}

func (rc *relayClient) getStatus(ctx context.Context) (*RelayStatus, error) {
	url := fmt.Sprintf("http://%s:%d/agent/relay/status", rc.host, rc.port)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if rc.token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+rc.token)
	}

	resp, err := rc.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var wrapper struct {
		Data *RelayStatus `json:"data"`
	}
	json.Unmarshal(body, &wrapper)
	return wrapper.Data, nil
}
