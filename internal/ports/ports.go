// Package ports wraps host port observation behind a swappable interface.
package ports

import "context"

// Range we care about; everything else is system or outbound noise.
const (
	MinPort = 3000
	MaxPort = 9999
)

// ListeningPort is one observed listening socket.
type ListeningPort struct {
	Port        int
	PID         int32  // 0 = unknown
	ProcessName string // "" = unknown; best-effort, never fails the scan
}

// Scanner lists current listening ports. Implementations (gopsutil, fake)
// stay behind this so callers never touch the lib directly.
type Scanner interface {
	ListeningPorts(ctx context.Context) ([]ListeningPort, error)
}

// InRange reports whether port is in the watched range.
func InRange(port int) bool { return port >= MinPort && port <= MaxPort }

// FilterRange keeps only ports in the watched range.
// ponytail: filter here so every Scanner stays dumb.
func FilterRange(in []ListeningPort) []ListeningPort {
	out := in[:0:0]
	for _, p := range in {
		if InRange(p.Port) {
			out = append(out, p)
		}
	}
	return out
}
