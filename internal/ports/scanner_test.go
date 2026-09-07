package ports

import (
	"context"
	"testing"

	gopsnet "github.com/shirou/gopsutil/v4/net"
)

func conn(port uint32, status string, pid int32) gopsnet.ConnectionStat {
	return gopsnet.ConnectionStat{Laddr: gopsnet.Addr{IP: "0.0.0.0", Port: port}, Status: status, Pid: pid}
}

func TestListeningPortsFiltersDedupesSorts(t *testing.T) {
	s := &GopsutilScanner{
		Connections: func(ctx context.Context) ([]gopsnet.ConnectionStat, error) {
			return []gopsnet.ConnectionStat{
				conn(3000, "LISTEN", 0),       // v4 without pid
				conn(3000, "LISTEN", 11),      // v6 duplicate with pid wins
				conn(80, "LISTEN", 12),        // out of range
				conn(4000, "ESTABLISHED", 13), // not listening
				conn(9999, "LISTEN", 14),      // upper edge kept
				conn(10000, "LISTEN", 15),     // above range
				conn(2999, "LISTEN", 16),      // below range
			}, nil
		},
		NameOf: func(ctx context.Context, pid int32) string {
			if pid == 11 {
				return "node"
			}
			return ""
		},
	}
	got, err := s.ListeningPorts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Port != 3000 || got[1].Port != 9999 {
		t.Fatalf("ports = %+v, want [3000 9999]", got)
	}
	if got[0].PID != 11 || got[0].ProcessName != "node" {
		t.Fatalf("port 3000 = %+v, want pid 11 node", got[0])
	}
}
