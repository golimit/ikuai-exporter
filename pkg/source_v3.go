package pkg

import (
	"fmt"
	"strings"
	"time"

	"github.com/jakeslee/ikuai"
	action3 "github.com/jakeslee/ikuai/action"
)

type sourceV3 struct {
	client *ikuai.IKuai
	major  int
}

func newSourceV3(url, username, password string, insecureSkip bool, timeout time.Duration) *sourceV3 {
	c := ikuai.NewIKuai(url, username, password, insecureSkip, true)
	c.SetTimeout(timeout)
	return &sourceV3{client: c, major: 3}
}

func (s *sourceV3) Major() int { return s.major }

func (s *sourceV3) SysStat() (*SysStatData, error) {
	stat, err := s.client.ShowSysStat()
	if err != nil {
		return nil, err
	}
	if !stat.Ok() {
		return nil, fmt.Errorf("sysstat not ok: %s", stat.ErrMsg)
	}
	sys := stat.Data.SysStat
	out := &SysStatData{
		Version:    sys.Verinfo.Version,
		Arch:       sys.Verinfo.Arch,
		Verstring:  sys.Verinfo.Verstring,
		CPUTemp:    sys.Cputemp,
		CPU:        sys.Cpu,
		MemTotal:   sys.Memory.Total,
		MemAvail:   sys.Memory.Available,
		MemCached:  sys.Memory.Cached,
		MemBuffers: sys.Memory.Buffers,
		OnlineUser: sys.OnlineUser.Count,
		Uptime:     int64(sys.Uptime),
		StreamUp:   sys.Stream.TotalUp,
		StreamDown: sys.Stream.TotalDown,
		Upload:     float64(sys.Stream.Upload),
		Download:   float64(sys.Stream.Download),
		ConnectNum: float64(sys.Stream.ConnectNum),
	}
	return out, nil
}

func (s *sourceV3) LanDevices() ([]DeviceData, error) {
	devices := map[string]DeviceData{}
	var errs []error

	appendFrom := func(items []action3.LanDeviceInfo) {
		for _, d := range items {
			id := d.IPAddr
			if _, ok := devices[id]; !ok {
				devices[id] = DeviceData{
					IPAddr:     d.IPAddr,
					MAC:        d.Mac,
					Hostname:   d.Hostname,
					Comment:    d.Comment,
					TotalUp:    d.TotalUp,
					TotalDown:  d.TotalDown,
					Upload:     float64(d.Upload),
					Download:   float64(d.Download),
					ConnectNum: float64(d.ConnectNum),
				}
			}
		}
	}

	lan, err := s.client.ShowMonitorLan()
	if err != nil || !lan.Ok() {
		errs = append(errs, fmt.Errorf("lanip: %w", err))
	} else {
		appendFrom(lan.Data.Data)
	}

	lan6, err := s.client.ShowMonitorLanIPv6()
	if err != nil || !lan6.Ok() {
		errs = append(errs, fmt.Errorf("lanipv6: %w", err))
	} else {
		appendFrom(lan6.Data.Data)
	}

	out := make([]DeviceData, 0, len(devices))
	for _, d := range devices {
		out = append(out, d)
	}
	if len(out) == 0 && len(errs) > 0 {
		return nil, errs[0]
	}
	return out, nil
}

func (s *sourceV3) Interfaces() ([]IfaceData, error) {
	info, err := s.client.ShowMonitorInterface()
	if err != nil {
		return nil, err
	}
	if !info.Ok() {
		return nil, fmt.Errorf("monitor_iface not ok: %s", info.ErrMsg)
	}

	rows := map[string]*IfaceData{}
	for _, iface := range info.Data.IfaceStream {
		rows[iface.Interface] = &IfaceData{
			Interface:  iface.Interface,
			Comment:    iface.Comment,
			IPAddr:     iface.IPAddr,
			TotalUp:    float64(iface.TotalUp),
			TotalDown:  float64(iface.TotalDown),
			Upload:     float64(iface.Upload),
			Download:   float64(iface.Download),
			ConnectNum: parseFloatString(iface.ConnectNum),
			Up:         true,
		}
	}

	for _, check := range info.Data.IfaceCheck {
		row, ok := rows[check.Interface]
		if !ok {
			row = &IfaceData{Interface: check.Interface, Up: true}
			rows[check.Interface] = row
		}
		row.Internet = check.Internet
		row.ParentInterface = check.ParentInterface
		if check.Result != "success" {
			row.Up = false
		} else if ts, err := parseInt64(check.Updatetime); err == nil {
			row.Uptime = time.Now().Unix() - ts
		}
	}

	out := make([]IfaceData, 0, len(rows))
	for _, r := range rows {
		out = append(out, *r)
	}
	return out, nil
}

func (s *sourceV3) DNAT() ([]DNATRule, error) {
	var raw map[string]interface{}
	_, err := s.client.Run(&action3.Action{
		Action:   "show",
		FuncName: "dnat",
		Param: map[string]interface{}{
			"TYPE":  "data,total",
			"limit": "0,10000",
		},
	}, &raw)
	if err != nil {
		return nil, err
	}
	return parseDNATFromMap(raw)
}

func (s *sourceV3) Sessions() ([]Session, error) {
	return nil, ErrSessionUnsupported
}

// lanConnItem is one connection row from v3 monitor_lanip TYPE=conn,conn_num.
// From the monitored LAN host's perspective: src_port is the local service port,
// dst_addr/dst_port is the remote peer.
type lanConnItem struct {
	Protocol string
	SrcPort  string
	DstAddr  string
	DstPort  string
}

func (s *sourceV3) fetchLanConnByIP(rules []DNATRule) (map[string][]lanConnItem, error) {
	connByIP := make(map[string][]lanConnItem)
	for _, rule := range rules {
		if !rule.Enabled || rule.LANAddr == "" {
			continue
		}
		if _, ok := connByIP[rule.LANAddr]; ok {
			continue
		}
		fetched, err := s.fetchLanConnections(rule.LANAddr)
		if err != nil {
			return nil, fmt.Errorf("monitor_lanip %s: %w", rule.LANAddr, err)
		}
		connByIP[rule.LANAddr] = fetched
	}
	return connByIP, nil
}

func (s *sourceV3) CountDNATConnections(rules []DNATRule, _ []Session) (map[int64]int, error) {
	connByIP, err := s.fetchLanConnByIP(rules)
	if err != nil {
		return nil, err
	}
	return countV3DNATFromLanIP(rules, connByIP), nil
}

func (s *sourceV3) ListDNATSessions(rules []DNATRule, _ []Session) ([]DNATSessionDetail, error) {
	connByIP, err := s.fetchLanConnByIP(rules)
	if err != nil {
		return nil, err
	}
	return matchV3DNATSessions(rules, connByIP), nil
}

func countV3DNATFromLanIP(rules []DNATRule, connByIP map[string][]lanConnItem) map[int64]int {
	return CountDNATConnectionsFromDetails(rules, matchV3DNATSessions(rules, connByIP))
}

func matchV3DNATSessions(rules []DNATRule, connByIP map[string][]lanConnItem) []DNATSessionDetail {
	out := make([]DNATSessionDetail, 0)
	for _, rule := range rules {
		if !rule.Enabled || rule.LANAddr == "" {
			continue
		}
		for _, c := range connByIP[rule.LANAddr] {
			if !protocolCompatible(rule.Protocol, c.Protocol) || !portMatches(rule.LANPort, c.SrcPort) {
				continue
			}
			out = append(out, DNATSessionDetail{
				Rule:    rule,
				SrcAddr: c.DstAddr,
				SrcPort: c.DstPort,
				DstAddr: rule.LANAddr,
				DstPort: c.SrcPort,
			})
		}
	}
	return out
}

func (s *sourceV3) fetchLanConnections(ip string) ([]lanConnItem, error) {
	var raw map[string]interface{}
	_, err := s.client.Run(&action3.Action{
		Action:   "show",
		FuncName: "monitor_lanip",
		Param: map[string]interface{}{
			"TYPE":      "conn,conn_num",
			"ip":        ip,
			"interface": "all",
			"proto":     "all",
			"maxnum":    500,
			"limit":     "0,500",
		},
	}, &raw)
	if err != nil {
		return nil, err
	}
	return parseLanConnectionsFromMap(raw)
}

func parseLanConnectionsFromMap(raw map[string]interface{}) ([]lanConnItem, error) {
	if raw == nil {
		return nil, fmt.Errorf("empty monitor_lanip response")
	}
	if msg, _ := raw["ErrMsg"].(string); msg != "" && msg != "Success" {
		return nil, fmt.Errorf("monitor_lanip error: %s", msg)
	}

	var payload map[string]interface{}
	if d, ok := raw["Data"].(map[string]interface{}); ok {
		payload = d
	} else if r, ok := raw["results"].(map[string]interface{}); ok {
		payload = r
	} else {
		return nil, fmt.Errorf("monitor_lanip missing data payload")
	}

	list, _ := payload["conn"].([]interface{})
	out := make([]lanConnItem, 0, len(list))
	for _, item := range list {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		out = append(out, lanConnItem{
			Protocol: strings.ToLower(fmt.Sprint(m["protocol"])),
			SrcPort:  anyToString(m["src_port"]),
			DstAddr:  anyToString(m["dst_addr"]),
			DstPort:  anyToString(m["dst_port"]),
		})
	}
	return out, nil
}

func toFloat(v interface{}) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int:
		return float64(t)
	case int64:
		return float64(t)
	case string:
		f, _ := parseFloat(t)
		return f
	default:
		return 0
	}
}

func parseFloat(s string) (float64, error) {
	var f float64
	_, err := fmt.Sscanf(s, "%g", &f)
	return f, err
}

func parseInt64(s string) (int64, error) {
	var n int64
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}
