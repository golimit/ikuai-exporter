package pkg

import (
	"testing"
)

func TestParseLanConnectionsFromMapV3Envelope(t *testing.T) {
	raw := map[string]interface{}{
		"Result": float64(30000),
		"ErrMsg": "Success",
		"Data": map[string]interface{}{
			"conn": []interface{}{
				map[string]interface{}{
					"protocol": "tcp",
					"src_port": float64(5173),
					"dst_addr": "192.168.85.101",
					"dst_port": float64(54131),
				},
				map[string]interface{}{
					"protocol": "tcp",
					"src_port": float64(5432),
					"dst_addr": "192.168.85.95",
					"dst_port": float64(40212),
				},
				map[string]interface{}{
					"protocol": "udp",
					"src_port": "42405",
					"dst_addr": "192.168.85.95",
					"dst_port": float64(35993),
				},
			},
			"conn_num": float64(0),
		},
	}

	conns, err := parseLanConnectionsFromMap(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(conns) != 3 {
		t.Fatalf("expected 3 connections, got %d", len(conns))
	}
	if conns[0].SrcPort != "5173" || conns[0].Protocol != "tcp" {
		t.Fatalf("unexpected first conn: %+v", conns[0])
	}
	if conns[2].SrcPort != "42405" || conns[2].Protocol != "udp" {
		t.Fatalf("unexpected third conn: %+v", conns[2])
	}
}

func TestCountV3DNATConnectionsFromLanIP(t *testing.T) {
	rules := []DNATRule{
		{ID: 1, Enabled: true, Protocol: "tcp", LANAddr: "192.168.80.210", LANPort: "5173"},
		{ID: 2, Enabled: true, Protocol: "tcp", LANAddr: "192.168.80.213", LANPort: "5432"},
		{ID: 3, Enabled: true, Protocol: "tcp+udp", LANAddr: "192.168.80.25", LANPort: "3000"},
		{ID: 4, Enabled: true, Protocol: "tcp", LANAddr: "192.168.80.25", LANPort: "3001"},
		{ID: 5, Enabled: false, Protocol: "tcp", LANAddr: "192.168.80.211", LANPort: "11002"},
	}

	lanIPConns := map[string][]lanConnItem{
		"192.168.80.210": {
			{Protocol: "tcp", SrcPort: "5173"},
			{Protocol: "tcp", SrcPort: "5173"},
			{Protocol: "tcp", SrcPort: "5173"},
		},
		"192.168.80.213": {
			{Protocol: "tcp", SrcPort: "5432"},
			{Protocol: "tcp", SrcPort: "5432"},
			{Protocol: "tcp", SrcPort: "5432"},
			{Protocol: "tcp", SrcPort: "5432"},
			{Protocol: "tcp", SrcPort: "5432"},
		},
		"192.168.80.25": {
			{Protocol: "tcp", SrcPort: "3000"},
			{Protocol: "udp", SrcPort: "42405"},
		},
	}

	counts := countV3DNATFromLanIP(rules, lanIPConns)
	if counts[1] != 3 {
		t.Fatalf("rule 1 expected 3, got %d", counts[1])
	}
	if counts[2] != 5 {
		t.Fatalf("rule 2 expected 5, got %d", counts[2])
	}
	if counts[3] != 1 {
		t.Fatalf("rule 3 expected 1, got %d", counts[3])
	}
	if counts[4] != 0 {
		t.Fatalf("rule 4 expected 0, got %d", counts[4])
	}
	if _, ok := counts[5]; ok {
		t.Fatal("disabled rule should not be in counts")
	}
}

func TestParseLanConnectionsFromMapError(t *testing.T) {
	if _, err := parseLanConnectionsFromMap(map[string]interface{}{
		"ErrMsg": "参数错误",
	}); err == nil {
		t.Fatal("expected error for failed envelope")
	}
}
