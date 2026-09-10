package pkg

import (
	"strconv"
	"strings"
)

// protocolCompatible reports whether a session protocol matches a DNAT rule protocol.
// Rule protocol "any" or empty matches any session protocol.
func protocolCompatible(ruleProto, sessionProto string) bool {
	ruleProto = strings.ToLower(strings.TrimSpace(ruleProto))
	sessionProto = strings.ToLower(strings.TrimSpace(sessionProto))
	if ruleProto == "" || ruleProto == "any" || ruleProto == "ip" {
		return true
	}
	return ruleProto == sessionProto
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
	if rule.LANAddr != "" && s.DSTAddr == rule.LANAddr && rule.LANPort != "" && s.DSTPort == rule.LANPort {
		return true
	}
	if rule.WANPort != "" && s.DSTPort == rule.WANPort && !isPrivateOrLocalIP(s.SrcAddr) {
		return true
	}
	return false
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
