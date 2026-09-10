package pkg

import (
	"strconv"
	"strings"
)

// protocolCompatible reports whether a session protocol matches a DNAT rule protocol.
// Rule protocol "any"/empty matches all; "tcp+udp" matches tcp or udp.
func protocolCompatible(ruleProto, sessionProto string) bool {
	ruleProto = strings.ToLower(strings.TrimSpace(ruleProto))
	sessionProto = strings.ToLower(strings.TrimSpace(sessionProto))
	if ruleProto == "" || ruleProto == "any" || ruleProto == "ip" {
		return true
	}
	if ruleProto == sessionProto {
		return true
	}
	if ruleProto == "tcp+udp" || ruleProto == "udp+tcp" {
		return sessionProto == "tcp" || sessionProto == "udp"
	}
	// session may be "tcp+udp" style too — accept if rule is contained
	if sessionProto == "tcp+udp" || sessionProto == "udp+tcp" {
		return ruleProto == "tcp" || ruleProto == "udp"
	}
	return false
}

// portMatches handles single ports and v3 ranges like "32080-32443".
func portMatches(rulePort, sessionPort string) bool {
	rulePort = strings.TrimSpace(rulePort)
	sessionPort = strings.TrimSpace(sessionPort)
	if rulePort == "" || sessionPort == "" || sessionPort == "--" {
		return false
	}
	if !strings.Contains(rulePort, "-") {
		return rulePort == sessionPort
	}
	parts := strings.SplitN(rulePort, "-", 2)
	if len(parts) != 2 {
		return rulePort == sessionPort
	}
	sp, err1 := strconv.Atoi(sessionPort)
	lo, err2 := strconv.Atoi(strings.TrimSpace(parts[0]))
	hi, err3 := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err1 != nil || err2 != nil || err3 != nil {
		return rulePort == sessionPort
	}
	if lo > hi {
		lo, hi = hi, lo
	}
	return sp >= lo && sp <= hi
}

// isPrivateOrLocalIP is a coarse filter for "not a public client".
func isPrivateOrLocalIP(ip string) bool {
	ip = strings.TrimSpace(ip)
	if ip == "" || ip == "--" {
		return true
	}
	// IPv6 ULA / link-local / loopback — treat as local for inbound matching.
	if strings.Contains(ip, ":") {
		lower := strings.ToLower(ip)
		return strings.HasPrefix(lower, "fe80:") ||
			strings.HasPrefix(lower, "fc") ||
			strings.HasPrefix(lower, "fd") ||
			lower == "::1"
	}
	parts := strings.Split(ip, ".")
	if len(parts) != 4 {
		return false
	}
	switch parts[0] {
	case "10", "127":
		return true
	case "192":
		return parts[1] == "168"
	case "172":
		second, err := strconv.Atoi(parts[1])
		if err != nil {
			return false
		}
		return second >= 16 && second <= 31
	case "169":
		return parts[1] == "254"
	default:
		return false
	}
}

// matchSessionToRule reports whether a session is attributed to the DNAT rule.
// Matching strategy (first hit wins, caller must not double-count):
//  1. post-DNAT: dst is lan_addr:lan_port
//  2. pre-DNAT:  dst_port is wan_port and src is not private/local
func matchSessionToRule(rule DNATRule, s Session) bool {
	if !protocolCompatible(rule.Protocol, s.Protocol) {
		return false
	}
	if rule.LANAddr != "" && s.DSTAddr == rule.LANAddr && portMatches(rule.LANPort, s.DSTPort) {
		return true
	}
	if portMatches(rule.WANPort, s.DSTPort) && !isPrivateOrLocalIP(s.SrcAddr) {
		return true
	}
	return false
}

// DNATSessionDetail is one inbound connection attributed to a DNAT rule.
type DNATSessionDetail struct {
	Rule    DNATRule
	SrcAddr string // inbound/client IP
	SrcPort string
	DstAddr string // internal target IP
	DstPort string
}

// sessionDetailForRule builds normalized addresses when a session matches a rule.
func sessionDetailForRule(rule DNATRule, s Session) (DNATSessionDetail, bool) {
	if !matchSessionToRule(rule, s) {
		return DNATSessionDetail{}, false
	}
	detail := DNATSessionDetail{Rule: rule}
	// post-DNAT: dst is the internal target.
	if rule.LANAddr != "" && s.DSTAddr == rule.LANAddr && portMatches(rule.LANPort, s.DSTPort) {
		detail.SrcAddr = s.SrcAddr
		detail.SrcPort = s.SrcPort
		detail.DstAddr = s.DSTAddr
		detail.DstPort = s.DSTPort
		return detail, true
	}
	// pre-DNAT: fill internal target from the rule.
	detail.SrcAddr = s.SrcAddr
	detail.SrcPort = s.SrcPort
	detail.DstAddr = rule.LANAddr
	detail.DstPort = rule.LANPort
	return detail, true
}

// MatchDNATSessions returns per-connection detail for sessions attributed to DNAT rules.
// Each session is attributed to at most one rule (first match in rule order).
func MatchDNATSessions(rules []DNATRule, sessions []Session) []DNATSessionDetail {
	out := make([]DNATSessionDetail, 0)
	for _, s := range sessions {
		for _, rule := range rules {
			if !rule.Enabled {
				continue
			}
			detail, ok := sessionDetailForRule(rule, s)
			if ok {
				out = append(out, detail)
				break
			}
		}
	}
	return out
}

// CountDNATConnectionsFromDetails returns per-rule counts from matched session details.
func CountDNATConnectionsFromDetails(rules []DNATRule, details []DNATSessionDetail) map[int64]int {
	counts := make(map[int64]int, len(rules))
	for _, rule := range rules {
		if rule.Enabled {
			counts[rule.ID] = 0
		}
	}
	for _, d := range details {
		counts[d.Rule.ID]++
	}
	return counts
}

// CountDNATConnections returns connection counts keyed by rule identity.
// Each session is attributed to at most one rule (first match in rule order).
func CountDNATConnections(rules []DNATRule, sessions []Session) map[int64]int {
	counts := make(map[int64]int, len(rules))
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		counts[rule.ID] = 0
	}
	for _, s := range sessions {
		for _, rule := range rules {
			if !rule.Enabled {
				continue
			}
			if matchSessionToRule(rule, s) {
				counts[rule.ID]++
				break
			}
		}
	}
	return counts
}

// truncateSessions caps the session list for high-cardinality detail export.
func truncateSessions(sessions []Session, limit int) []Session {
	if limit <= 0 || len(sessions) <= limit {
		return sessions
	}
	return sessions[:limit]
}

// truncateDNATSessions caps DNAT session detail for export.
func truncateDNATSessions(details []DNATSessionDetail, limit int) []DNATSessionDetail {
	if limit <= 0 || len(details) <= limit {
		return details
	}
	return details[:limit]
}
