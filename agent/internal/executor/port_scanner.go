package executor

type PortScanner struct{}

func NewPortScanner() *PortScanner {
	return &PortScanner{}
}

func MySQLReservedPortFor(port int) string {
	switch port {
	case 33061:
		return "mgr"
	case 4567, 4568, 4444:
		return "pxc"
	default:
		return ""
	}
}

func IsMySQLReservedPort(port int) bool {
	return MySQLReservedPortFor(port) != ""
}

func ExcludeMySQLReservedPorts(ports []int) []int {
	var out []int
	for _, p := range ports {
		if !IsMySQLReservedPort(p) {
			out = append(out, p)
		}
	}
	return out
}

func (s *PortScanner) FindAvailablePort(startPort int, exclude []int) (int, error) {
	if startPort < 1 || startPort > 65535 {
		return 0, &portRangeError{port: startPort}
	}
	excludeSet := make(map[int]bool)
	for _, p := range exclude {
		excludeSet[p] = true
	}
	for port := startPort; port <= 65535; port++ {
		if IsMySQLReservedPort(port) {
			continue
		}
		if excludeSet[port] {
			continue
		}
		return port, nil
	}
	return 0, &portRangeError{port: startPort}
}

func (s *PortScanner) FindAvailableDataDir(basePath string, port int) string {
	return basePath + "/mysql_" + itoa(port)
}

type portRangeError struct {
	port int
}

func (e *portRangeError) Error() string {
	return "port out of range: " + itoa(e.port)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	if neg {
		s = "-" + s
	}
	return s
}
