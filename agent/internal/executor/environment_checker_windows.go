//go:build windows

package executor

import (
	"syscall"
	"unsafe"
)

var (
	kernel32        = syscall.NewLazyDLL("kernel32.dll")
	getDiskFreeExW  = kernel32.NewProc("GetDiskFreeSpaceExW")
	getMemStatusEx  = kernel32.NewProc("GlobalMemoryStatusEx")
)

// memoryStatusEx 对应 Windows MEMORYSTATUSEX 结构体
type memoryStatusEx struct {
	dwLength                uint32
	dwMemoryLoad            uint32
	ullTotalPhys            uint64
	ullAvailPhys            uint64
	ullTotalPageFile        uint64
	ullAvailPageFile        uint64
	ullTotalVirtual         uint64
	ullAvailVirtual         uint64
	ullAvailExtendedVirtual uint64
}

// checkResourcesPlatform 检查系统资源（Windows平台实现）
func (e *EnvironmentChecker) checkResourcesPlatform(info *ResourcesInfo) {
	info.MemoryMB = getWindowsMemoryMB()
	info.DiskSpaceGB = getWindowsDiskSpaceGB()
}

// getWindowsMemoryMB 使用 GlobalMemoryStatusEx 获取内存（MB）
func getWindowsMemoryMB() int {
	var memInfo memoryStatusEx
	memInfo.dwLength = uint32(unsafe.Sizeof(memInfo))

	ret, _, _ := getMemStatusEx.Call(uintptr(unsafe.Pointer(&memInfo)))
	if ret != 0 {
		return int(memInfo.ullTotalPhys / (1024 * 1024))
	}
	return 0
}

// getWindowsDiskSpaceGB 使用 GetDiskFreeSpaceExW 获取 C 盘可用空间（GB）
func getWindowsDiskSpaceGB() int {
	var freeBytesAvailable uint64
	var totalBytes uint64
	var totalFreeBytes uint64

	path, _ := syscall.UTF16PtrFromString(`C:\`)
	ret, _, _ := getDiskFreeExW.Call(
		uintptr(unsafe.Pointer(path)),
		uintptr(unsafe.Pointer(&freeBytesAvailable)),
		uintptr(unsafe.Pointer(&totalBytes)),
		uintptr(unsafe.Pointer(&totalFreeBytes)),
	)
	if ret != 0 {
		return int(freeBytesAvailable / (1024 * 1024 * 1024))
	}
	return 0
}
