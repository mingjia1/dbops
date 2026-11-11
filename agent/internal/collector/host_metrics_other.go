//go:build !linux

package collector

// 非 Linux 平台 (本地 Windows 开发/测试) 不采集主机资源指标,
// 返回空序列即可, 不影响 MySQL status 指标链路.
func collectHostResourceMetrics(instanceID string) []Metric {
	return nil
}
