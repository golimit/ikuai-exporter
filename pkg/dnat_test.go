package pkg

import (
	"encoding/json"
	"testing"
)

func TestParseDNATRulesFromLiveShape(t *testing.T) {
	raw := `{
	  "code": 0,
	  "message": "Success",
	  "results": {
	    "data": [
	      {
	        "id": 1,
	        "enabled": "yes",
	        "tagname": "newapi",
	        "comment": "",
	        "interface": "192.168.31.250",
	        "src_addr": {"object":{},"custom":{}},
	        "lan_addr": "192.168.80.25",
	        "protocol": "tcp",
	        "wan_port": "3000",
	        "lan_port": "3000",
	        "lan_addr_int": 3232256025
	      }
	    ],
	    "total": 1,
	    "enabled_total": 1,
	    "disabled_total": 0
	  }
	}`

	var result showDNATResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !result.Ok() {
		t.Fatalf("expected ok status")
	}

	rules := ParseDNATRules(result.Results.Data)
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	r := rules[0]
	if !r.Enabled || r.Tagname != "newapi" || r.LANAddr != "192.168.80.25" ||
		r.WANPort != "3000" || r.LANPort != "3000" || r.Protocol != "tcp" ||
		r.Interface != "192.168.31.250" {
		t.Fatalf("unexpected rule: %+v", r)
	}
}

func TestParseSessionsNumericPorts(t *testing.T) {
	raw := `{
	  "code": 0,
	  "message": "Success",
	  "results": {
	    "conn": [
	      {
	        "protocol": "udp",
	        "src_addr": "192.168.31.201",
	        "src_port": 42223,
	        "dst_addr": "8.8.8.8",
	        "dst_port": "53",
	        "terminal_addr": "192.168.31.201",
	        "terminal_port": 42223,
	        "app_name": "DNS",
	        "total_up": 10,
	        "total_down": 20
	      }
	    ]
	  }
	}`
	var result showCollectConnResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	sessions := ParseSessions(result.Results.Conn)
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	s := sessions[0]
	if s.SrcPort != "42223" || s.DSTPort != "53" || s.TerminalPort != "42223" {
		t.Fatalf("port stringify failed: %+v", s)
	}
	if s.TotalUp != 10 || s.TotalDown != 20 {
		t.Fatalf("traffic fields: %+v", s)
	}
}

func TestCountDNATConnections(t *testing.T) {
	rules := []DNATRule{
		{ID: 1, Enabled: true, Protocol: "tcp", WANPort: "3000", LANAddr: "192.168.80.25", LANPort: "3000"},
		{ID: 2, Enabled: false, Protocol: "udp", WANPort: "53", LANAddr: "192.168.80.26", LANPort: "53"},
	}
	sessions := []Session{
		// post-DNAT match
		{Protocol: "tcp", SrcAddr: "1.2.3.4", SrcPort: "52133", DSTAddr: "192.168.80.25", DSTPort: "3000"},
		// pre-DNAT match (public src, wan port)
		{Protocol: "tcp", SrcAddr: "8.8.8.8", SrcPort: "61231", DSTAddr: "192.168.31.250", DSTPort: "3000"},
		// outbound unrelated
		{Protocol: "tcp", SrcAddr: "192.168.31.201", SrcPort: "44602", DSTAddr: "1.1.1.1", DSTPort: "443"},
		// protocol mismatch
		{Protocol: "udp", SrcAddr: "9.9.9.9", SrcPort: "1111", DSTAddr: "192.168.80.25", DSTPort: "3000"},
	}

	counts := CountDNATConnections(rules, sessions)
	if counts[1] != 2 {
		t.Fatalf("rule1 expected 2, got %d", counts[1])
	}
	if _, ok := counts[2]; ok {
		t.Fatalf("disabled rule should not be in counts map")
	}
}

func TestMatchSessionDoesNotDoubleCountWhenRuleOrderPicksFirst(t *testing.T) {
	rules := []DNATRule{
		{ID: 1, Enabled: true, Protocol: "any", WANPort: "3000", LANAddr: "10.0.0.5", LANPort: "3000"},
		{ID: 2, Enabled: true, Protocol: "any", WANPort: "3000", LANAddr: "10.0.0.6", LANPort: "3000"},
	}
	sessions := []Session{
		{Protocol: "tcp", SrcAddr: "1.2.3.4", DSTAddr: "10.0.0.5", DSTPort: "3000"},
	}
	counts := CountDNATConnections(rules, sessions)
	if counts[1] != 1 || counts[2] != 0 {
		t.Fatalf("expected first-rule attribution, got %+v", counts)
	}
}

func TestIsPrivateOrLocalIP(t *testing.T) {
	private := []string{"10.0.0.1", "192.168.1.1", "172.16.0.1", "172.31.255.1", "127.0.0.1", "fe80::1"}
	for _, ip := range private {
		if !isPrivateOrLocalIP(ip) {
			t.Fatalf("expected private: %s", ip)
		}
	}
	public := []string{"1.2.3.4", "8.8.8.8", "172.15.0.1", "172.32.0.1", "172.2.0.1", "172.200.0.1", "11.0.0.1"}
	for _, ip := range public {
		if isPrivateOrLocalIP(ip) {
			t.Fatalf("expected public: %s", ip)
		}
	}
}

func TestMatchDNATSessionsPostDNAT(t *testing.T) {
	rules := []DNATRule{
		{ID: 1, Enabled: true, Protocol: "tcp", WANPort: "3000", LANAddr: "192.168.80.25", LANPort: "3000", Tagname: "svc"},
	}
	sessions := []Session{
		{Protocol: "tcp", SrcAddr: "1.2.3.4", SrcPort: "52133", DSTAddr: "192.168.80.25", DSTPort: "3000"},
	}
	details := MatchDNATSessions(rules, sessions)
	if len(details) != 1 {
		t.Fatalf("expected 1 detail, got %d", len(details))
	}
	d := details[0]
	if d.SrcAddr != "1.2.3.4" || d.SrcPort != "52133" || d.DstAddr != "192.168.80.25" || d.DstPort != "3000" {
		t.Fatalf("unexpected post-DNAT detail: %+v", d)
	}
}

func TestMatchDNATSessionsPreDNAT(t *testing.T) {
	rules := []DNATRule{
		{ID: 1, Enabled: true, Protocol: "tcp", WANPort: "3000", LANAddr: "192.168.80.25", LANPort: "3000", Tagname: "svc"},
	}
	sessions := []Session{
		{Protocol: "tcp", SrcAddr: "8.8.8.8", SrcPort: "61231", DSTAddr: "192.168.31.250", DSTPort: "3000"},
	}
	details := MatchDNATSessions(rules, sessions)
	if len(details) != 1 {
		t.Fatalf("expected 1 detail, got %d", len(details))
	}
	d := details[0]
	if d.SrcAddr != "8.8.8.8" || d.DstAddr != "192.168.80.25" || d.DstPort != "3000" {
		t.Fatalf("unexpected pre-DNAT detail: %+v", d)
	}
}

func TestCountDNATConnectionsFromDetails(t *testing.T) {
	rules := []DNATRule{
		{ID: 1, Enabled: true, Tagname: "a"},
		{ID: 2, Enabled: true, Tagname: "b"},
	}
	details := []DNATSessionDetail{
		{Rule: rules[0]},
		{Rule: rules[0]},
		{Rule: rules[1]},
	}
	counts := CountDNATConnectionsFromDetails(rules, details)
	if counts[1] != 2 || counts[2] != 1 {
		t.Fatalf("unexpected counts: %+v", counts)
	}
}

func TestTruncateSessions(t *testing.T) {
	sessions := make([]Session, 5)
	got := truncateSessions(sessions, 2)
	if len(got) != 2 {
		t.Fatalf("expected 2, got %d", len(got))
	}
	got = truncateSessions(sessions, 10)
	if len(got) != 5 {
		t.Fatalf("expected full slice, got %d", len(got))
	}
	got = truncateSessions(sessions, 0)
	if len(got) != 5 {
		t.Fatalf("non-positive limit should keep all, got %d", len(got))
	}
}
