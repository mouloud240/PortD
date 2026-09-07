package ports

import (
	"context"
	"sort"

	gopsnet "github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/process"
)

// GopsutilScanner is the production Scanner backed by gopsutil.
// Func fields default to live calls; tests inject fakes.
type GopsutilScanner struct {
	Connections func(ctx context.Context) ([]gopsnet.ConnectionStat, error)
	NameOf      func(ctx context.Context, pid int32) string
}

// NewGopsutilScanner returns a Scanner using live host data.
func NewGopsutilScanner() *GopsutilScanner { return &GopsutilScanner{} }

func (s *GopsutilScanner) ListeningPorts(ctx context.Context) ([]ListeningPort, error) {
	connsFn := s.Connections
	if connsFn == nil {
		connsFn = func(ctx context.Context) ([]gopsnet.ConnectionStat, error) {
			return gopsnet.ConnectionsWithContext(ctx, "tcp")
		}
	}
	nameOf := s.NameOf
	if nameOf == nil {
		nameOf = liveProcessName
	}
	conns, err := connsFn(ctx)
	if err != nil {
		return nil, err
	}
	// ponytail: dedupe by port (v4+v6 sockets share it); prefer entry with PID/name.
	merged := make(map[int]ListeningPort, len(conns))
	for _, c := range conns {
		if c.Status != "LISTEN" || !InRange(int(c.Laddr.Port)) {
			continue
		}
		port := int(c.Laddr.Port)
		name := ""
		if c.Pid > 0 {
			name = nameOf(ctx, c.Pid)
		}
		cur, ok := merged[port]
		if !ok || (cur.PID == 0 && c.Pid > 0) || (cur.ProcessName == "" && name != "") {
			merged[port] = ListeningPort{Port: port, PID: c.Pid, ProcessName: name}
		}
	}
	out := make([]ListeningPort, 0, len(merged))
	for _, p := range merged {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Port < out[j].Port })
	return out, nil
}

// liveProcessName resolves PID to name; "" on any failure by design.
func liveProcessName(ctx context.Context, pid int32) string {
	p, err := process.NewProcess(pid)
	if err != nil || p == nil {
		return ""
	}
	name, err := p.NameWithContext(ctx)
	if err != nil {
		return ""
	}
	return name
}
