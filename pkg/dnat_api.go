package pkg

import (
	"fmt"
	"strconv"
	"strings"

	action "github.com/jakeslee/ikuai/action"
	"github.com/jakeslee/ikuai/base"
)

// DNATRule is the normalized port-mapping rule from iKuai v4 func_name=dnat.
type DNATRule struct {
	ID        int64
	Enabled   bool
	Tagname   string
	Comment   string
	Interface string // WAN IP address, not iface name
	LANAddr   string
	LANPort   string
	WANPort   string
	Protocol  string
}

// Session is one entry from iKuai v4 func_name=collect_conn.
type Session struct {
	Protocol     string
	SrcAddr      string
	SrcPort      string
	DSTAddr      string
	DSTPort      string
	TerminalAddr string
	TerminalPort string
	SrcInterface string
	DSTInterface string
	AppName      string
	Domain       string
	TotalUp      float64
	TotalDown    float64
}

// Status mirrors the v4 Action/call envelope.
type Status struct {
	Result  int    `json:"Result"`
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (r *Status) Ok() bool {
	return r.Code == 0
}

type dnatAPIItem struct {
	ID        int64       `json:"id"`
	Enabled   string      `json:"enabled"`
	Tagname   string      `json:"tagname"`
	Comment   string      `json:"comment"`
	Interface string      `json:"interface"`
	LANAddr   string      `json:"lan_addr"`
	LANPort   interface{} `json:"lan_port"`
	WANPort   interface{} `json:"wan_port"`
	Protocol  string      `json:"protocol"`
}

type showDNATResult struct {
	Status
	Results struct {
		Data          []dnatAPIItem `json:"data"`
		Total         int64         `json:"total"`
		EnabledTotal  int64         `json:"enabled_total"`
		DisabledTotal int64         `json:"disabled_total"`
	} `json:"results"`
}

type collectConnItem struct {
	Protocol     string      `json:"protocol"`
	Status       string      `json:"status"`
	Starttime    interface{} `json:"starttime"`
	SrcAddr      string      `json:"src_addr"`
	SrcPort      interface{} `json:"src_port"`
	DSTAddr      string      `json:"dst_addr"`
	DSTPort      interface{} `json:"dst_port"`
	TerminalAddr string      `json:"terminal_addr"`
	TerminalPort interface{} `json:"terminal_port"`
	SrcInterface string      `json:"src_interface"`
	DSTInterface string      `json:"dst_interface"`
	AppName      string      `json:"app_name"`
	Domain       string      `json:"domain"`
	TotalUp      float64     `json:"total_up"`
	TotalDown    float64     `json:"total_down"`
}

type showCollectConnResult struct {
	Status
	Results struct {
		Conn []collectConnItem `json:"conn"`
	} `json:"results"`
}

// ShowDNAT fetches port-mapping rules via Action/call.
func ShowDNAT(i *base.IKuaiBase) (*showDNATResult, error) {
	resp := &showDNATResult{}
	_, err := i.Run(&action.Action{
		Action:   "show",
		FuncName: "dnat",
		Param: map[string]interface{}{
			"TYPE":  "data,total,enabled_total,disabled_total",
			"limit": "0,10000",
		},
	}, resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// ShowCollectConn fetches the current connection table via Action/call.
func ShowCollectConn(i *base.IKuaiBase) (*showCollectConnResult, error) {
	resp := &showCollectConnResult{}
	_, err := i.Run(&action.Action{
		Action:   "show",
		FuncName: "collect_conn",
		Param: map[string]interface{}{
			"TYPE": "all",
		},
	}, resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// ParseDNATRules converts API items into normalized rules.
func ParseDNATRules(items []dnatAPIItem) []DNATRule {
	rules := make([]DNATRule, 0, len(items))
	for _, item := range items {
		rules = append(rules, DNATRule{
			ID:        item.ID,
			Enabled:   strings.EqualFold(item.Enabled, "yes"),
			Tagname:   item.Tagname,
			Comment:   item.Comment,
			Interface: item.Interface,
			LANAddr:   item.LANAddr,
			LANPort:   anyToString(item.LANPort),
			WANPort:   anyToString(item.WANPort),
			Protocol:  strings.ToLower(item.Protocol),
		})
	}
	return rules
}

// ParseSessions converts API items into normalized sessions.
func ParseSessions(items []collectConnItem) []Session {
	sessions := make([]Session, 0, len(items))
	for _, item := range items {
		sessions = append(sessions, Session{
			Protocol:     strings.ToLower(item.Protocol),
			SrcAddr:      item.SrcAddr,
			SrcPort:      anyToString(item.SrcPort),
			DSTAddr:      item.DSTAddr,
			DSTPort:      anyToString(item.DSTPort),
			TerminalAddr: item.TerminalAddr,
			TerminalPort: anyToString(item.TerminalPort),
			SrcInterface: item.SrcInterface,
			DSTInterface: item.DSTInterface,
			AppName:      item.AppName,
			Domain:       item.Domain,
			TotalUp:      item.TotalUp,
			TotalDown:    item.TotalDown,
		})
	}
	return sessions
}

func anyToString(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", t))
	}
}
