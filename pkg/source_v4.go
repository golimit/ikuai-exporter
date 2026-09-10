package pkg

import (
	"fmt"
	"time"

	"github.com/jakeslee/ikuai"
	v4 "github.com/jakeslee/ikuai/v4"
	action_v4 "github.com/jakeslee/ikuai/v4/action"
)

type sourceV4 struct {
	client *v4.IKuaiV4
	major  int
}

func newSourceV4(client *v4.IKuaiV4) *sourceV4 {
	return &sourceV4{client: client, major: 4}
}

func newSourceV4FromConfig(url, username, password string, insecureSkip bool, timeout time.Duration) *sourceV4 {
	c := ikuai.NewV4(url, username, password, insecureSkip, true)
	c.SetTimeout(timeout)
	return newSourceV4(c)
}

func (s *sourceV4) Major() int { return s.major }

func (s *sourceV4) majorVersion() (int, error) {
	stat, err := s.client.ShowSysStat()
	if err != nil {
		return 0, err
	}
	if !stat.Ok() {
		return 0, fmt.Errorf("sysstat not ok")
	}
	return ParseMajor(stat.Results.SysStat.Verinfo.Version)
}

func (s *sourceV4) SysStat() (*SysStatData, error) {
	stat, err := s.client.ShowSysStat()
	if err != nil {
		return nil, err
	}
	if !stat.Ok() {
		return nil, fmt.Errorf("sysstat not ok")
	}
	sys := stat.Results.SysStat
	return &SysStatData{
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
	}, nil
}

func (s *sourceV4) LanDevices() ([]DeviceData, error) {
	devices := map[string]DeviceData{}
	var errs []error

	appendFrom := func(items []action_v4.LanDeviceInfo) {
		for _, d := range items {
			if _, ok := devices[d.IPAddr]; !ok {
				devices[d.IPAddr] = DeviceData{
					IPAddr:     d.IPAddr,
					MAC:        d.MAC,
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
		appendFrom(lan.Results.Data)
	}

	lan6, err := s.client.ShowMonitorLanIPv6()
	if err != nil || !lan6.Ok() {
		errs = append(errs, fmt.Errorf("lanipv6: %w", err))
	} else {
		appendFrom(lan6.Results.Data)
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

func (s *sourceV4) Interfaces() ([]IfaceData, error) {
	info, err := s.client.ShowMonitorInterface()
	if err != nil {
		return nil, err
	}
	if !info.Ok() {
		return nil, fmt.Errorf("monitor_iface not ok")
	}

	rows := map[string]*IfaceData{}
	for _, iface := range info.Results.IfaceStream {
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

	for _, check := range info.Results.IfaceCheck {
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

func (s *sourceV4) DNAT() ([]DNATRule, error) {
	result, err := ShowDNAT(s.client.IKuaiBase)
	if err != nil {
		return nil, err
	}
	if !result.Ok() {
		return nil, fmt.Errorf("dnat not ok: %+v", result.Status)
	}
	return ParseDNATRules(result.Results.Data), nil
}

func (s *sourceV4) Sessions() ([]Session, error) {
	result, err := ShowCollectConn(s.client.IKuaiBase)
	if err != nil {
		return nil, err
	}
	if !result.Ok() {
		return nil, fmt.Errorf("collect_conn not ok: %+v", result.Status)
	}
	return ParseSessions(result.Results.Conn), nil
}

func parseFloatString(s string) float64 {
	f, err := parseFloat(s)
	if err != nil {
		return 0
	}
	return f
}
