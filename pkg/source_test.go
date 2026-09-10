package pkg

import (
	"errors"
	"testing"
)

func TestParseMajor(t *testing.T) {
	cases := map[string]int{
		"3.7.15": 3,
		"4.0.1":  4,
		"4.12":   4,
		" 3.6 ":  3,
	}
	for in, want := range cases {
		got, err := ParseMajor(in)
		if err != nil || got != want {
			t.Fatalf("ParseMajor(%q)=%d,%v want %d", in, got, err, want)
		}
	}
	if _, err := ParseMajor(""); err == nil {
		t.Fatal("expected error for empty version")
	}
}

func TestProtocolCompatibleTCPUDP(t *testing.T) {
	if !protocolCompatible("tcp+udp", "tcp") || !protocolCompatible("tcp+udp", "udp") {
		t.Fatal("tcp+udp should match tcp and udp")
	}
	if protocolCompatible("tcp+udp", "icmp") {
		t.Fatal("tcp+udp should not match icmp")
	}
	if !protocolCompatible("TCP", "tcp") {
		t.Fatal("case insensitive match failed")
	}
}

func TestPortMatchesRange(t *testing.T) {
	if !portMatches("32080-32443", "32091") {
		t.Fatal("32091 should be inside 32080-32443")
	}
	if portMatches("32080-32443", "80") {
		t.Fatal("80 should be outside range")
	}
	if !portMatches("443", "443") {
		t.Fatal("single port equality failed")
	}
	if portMatches("443", "444") {
		t.Fatal("single port inequality failed")
	}
}

func TestParseDNATFromMapV3Envelope(t *testing.T) {
	raw := map[string]interface{}{
		"Result":  float64(30000),
		"ErrMsg":  "Success",
		"Data": map[string]interface{}{
			"data": []interface{}{
				map[string]interface{}{
					"id":        float64(23),
					"enabled":   "no",
					"comment":   "headscale",
					"interface": "wan1",
					"lan_addr":  "192.168.80.25",
					"lan_port":  "32080-32443",
					"wan_port":  "32080-32443",
					"protocol":  "tcp+udp",
				},
			},
			"total": float64(1),
		},
	}
	rules, err := parseDNATFromMap(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 1 {
		t.Fatalf("want 1 rule, got %d", len(rules))
	}
	r := rules[0]
	if r.ID != 23 || r.Enabled || r.Protocol != "tcp+udp" || r.WANPort != "32080-32443" {
		t.Fatalf("unexpected rule %+v", r)
	}
	if r.Tagname != "headscale" {
		t.Fatalf("tagname should fall back to comment, got %q", r.Tagname)
	}
}

func TestParseDNATFromMapV4Envelope(t *testing.T) {
	raw := map[string]interface{}{
		"code":    float64(0),
		"message": "Success",
		"results": map[string]interface{}{
			"data": []interface{}{
				map[string]interface{}{
					"id":        float64(1),
					"enabled":   "yes",
					"tagname":   "newapi",
					"interface": "192.168.31.250",
					"lan_addr":  "192.168.80.25",
					"lan_port":  float64(3000),
					"wan_port":  "3000",
					"protocol":  "tcp",
				},
			},
		},
	}
	rules, err := parseDNATFromMap(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 1 || rules[0].LANPort != "3000" || !rules[0].Enabled {
		t.Fatalf("unexpected %+v", rules)
	}
}

func TestParseDNATFromMapError(t *testing.T) {
	if _, err := parseDNATFromMap(map[string]interface{}{
		"ErrMsg": "Not found",
	}); err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(ErrSessionUnsupported, ErrSessionUnsupported) {
		t.Fatal("sanity")
	}
}
