package pkg

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jakeslee/ikuai"
	"github.com/sirupsen/logrus"
)

// ErrSessionUnsupported is returned by v3 devices that have no session table API.
var ErrSessionUnsupported = errors.New("session API not available on this iKuai version")

// SysStatData is the normalized system snapshot used by collectors.
type SysStatData struct {
	Version    string
	Arch       string
	Verstring  string
	CPUTemp    []int
	CPU        []string
	MemTotal   int64
	MemAvail   int64
	MemCached  int64
	MemBuffers int64
	OnlineUser int
	Uptime     int64
	StreamUp   int64
	StreamDown int64
	Upload     float64
	Download   float64
	ConnectNum float64
}

// DeviceData is a normalized LAN terminal.
type DeviceData struct {
	IPAddr     string
	MAC        string
	Hostname   string
	Comment    string
	TotalUp    float64
	TotalDown  float64
	Upload     float64
	Download   float64
	ConnectNum float64
}

// IfaceData is a normalized network interface row.
type IfaceData struct {
	Interface       string
	Comment         string
	Internet        string
	ParentInterface string
	IPAddr          string
	Up              bool
	Uptime          int64
	TotalUp         float64
	TotalDown       float64
	Upload          float64
	Download        float64
	ConnectNum      float64
}

// Source abstracts iKuai 3.x / 4.x data access for collectors.
type Source interface {
	Major() int
	SysStat() (*SysStatData, error)
	LanDevices() ([]DeviceData, error)
	Interfaces() ([]IfaceData, error)
	DNAT() ([]DNATRule, error)
	Sessions() ([]Session, error)
	// CountDNATConnections returns per-rule connection counts.
	// sessions is used by v4 (collect_conn); v3 ignores it and uses monitor_lanip.
	CountDNATConnections(rules []DNATRule, sessions []Session) (map[int64]int, error)
	// ListDNATSessions returns per-connection detail for DNAT-matched inbound sessions.
	ListDNATSessions(rules []DNATRule, sessions []Session) ([]DNATSessionDetail, error)
}

// ParseMajor extracts the major version from a version string like "3.7.15" or "4.0.1".
func ParseMajor(version string) (int, error) {
	version = strings.TrimSpace(version)
	if version == "" {
		return 0, fmt.Errorf("empty version")
	}
	parts := strings.Split(version, ".")
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, fmt.Errorf("parse major from %q: %w", version, err)
	}
	return major, nil
}

// NewSource detects the iKuai major version and returns a matching Source.
func NewSource(url, username, password string, insecureSkip bool, timeout time.Duration) (Source, error) {
	url = strings.TrimSpace(url)

	// Prefer v3 probe: login+sysstat works on 3.7.x and is cheap.
	i3 := ikuai.NewIKuai(url, username, password, insecureSkip, true)
	i3.SetTimeout(timeout)
	if _, err := i3.Login(); err == nil {
		if stat, err := i3.ShowSysStat(); err == nil && stat.Ok() {
			major, pErr := ParseMajor(stat.Data.SysStat.Verinfo.Version)
			if pErr == nil && major <= 3 {
				logrus.WithFields(logrus.Fields{
					"version": stat.Data.SysStat.Verinfo.Version,
					"major":   major,
				}).Info("detected iKuai 3.x, using v3 client")
				return newSourceV3(url, username, password, insecureSkip, timeout), nil
			}
			if pErr == nil && major >= 4 {
				// Some 4.x devices may still answer the v3 SDK path; fall through to v4.
				logrus.WithField("version", stat.Data.SysStat.Verinfo.Version).Info("detected iKuai 4.x via v3 probe, using v4 client")
			}
		}
	}

	i4 := ikuai.NewV4(url, username, password, insecureSkip, true)
	i4.SetTimeout(timeout)
	src := newSourceV4(i4)
	if major, err := src.majorVersion(); err == nil {
		logrus.WithField("major", major).Info("using iKuai v4 client")
	} else {
		logrus.WithError(err).Warn("failed to read iKuai version, defaulting to v4 client")
	}
	return src, nil
}
