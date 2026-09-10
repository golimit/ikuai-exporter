package pkg

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/samber/lo"
	"github.com/sirupsen/logrus"
)

var supported_modules = []string{
	"sysStat",
	"lanDevice",
	"interfaceInfo",
	"dnat",
	"session",
}

type IKuaiExporter struct {
	source             Source
	modules            []string
	sessionDetail      bool
	sessionDetailLimit int

	versionDesc *prometheus.Desc // ikuai 版本

	// CPU
	cpuUsageRatioDesc *prometheus.Desc // CPU 使用
	cpuTempDesc       *prometheus.Desc // CPU 温度

	// 内存
	memSizeDesc    *prometheus.Desc // 内存指标
	memUsageDesc   *prometheus.Desc // 内存指标
	memCachedDesc  *prometheus.Desc // 内存指标
	memBuffersDesc *prometheus.Desc // 内存指标

	// 终端
	lanDeviceDesc      *prometheus.Desc // 内网终端信息
	lanDeviceCountDesc *prometheus.Desc // 内网终端数量
	ifaceInfoDesc      *prometheus.Desc // 接口信息
	UpDesc             *prometheus.Desc // 在线状态，host/link
	UpTimeDesc         *prometheus.Desc // 在线时间，host/link

	// 网络，device/host/iface
	streamUpBytesDesc   *prometheus.Desc // 流量上行数据包
	streamDownBytesDesc *prometheus.Desc // 流量上行数据包
	streamUpSpeedDesc   *prometheus.Desc // 流量上行速度
	streamDownSpeedDesc *prometheus.Desc // 流量上行速度
	connCountDesc       *prometheus.Desc // 连接数指标

	// DNAT / Session
	dnatInfoDesc           *prometheus.Desc
	dnatTotalDesc          *prometheus.Desc
	dnatEnabledTotalDesc   *prometheus.Desc
	dnatDisabledTotalDesc  *prometheus.Desc
	dnatConnectionsDesc    *prometheus.Desc
	sessionTotalDesc       *prometheus.Desc
	sessionInfoDesc        *prometheus.Desc

	// Exporter
	MetricErrorDesc *prometheus.Desc // 指标获取报错
}

func NewIKuaiExporter(src Source, modules []string, sessionDetail bool, sessionDetailLimit int) *IKuaiExporter {
	usedModules := lo.Intersect(modules, supported_modules)

	if len(usedModules) == 0 {
		usedModules = supported_modules
	}

	logrus.WithFields(logrus.Fields{
		"supported":  strings.Join(supported_modules, ","),
		"configured": strings.Join(modules, ","),
		"used":       strings.Join(usedModules, ","),
		"major":      src.Major(),
	}).Info("init exporter modules")

	if sessionDetailLimit <= 0 {
		sessionDetailLimit = 200
	}

	return &IKuaiExporter{
		source:             src,
		modules:            usedModules,
		sessionDetail:      sessionDetail,
		sessionDetailLimit: sessionDetailLimit,
		versionDesc: prometheus.NewDesc("ikuai_version", "IKuai version info",
			[]string{"version", "arch", "verstring"}, nil),
		cpuUsageRatioDesc: prometheus.NewDesc("ikuai_cpu_usage_ratio", "IKuai CPU usage ratio",
			[]string{"id"}, nil),
		cpuTempDesc: prometheus.NewDesc("ikuai_cpu_temperature", "",
			nil, nil),
		memSizeDesc: prometheus.NewDesc("ikuai_memory_size_bytes", "",
			[]string{}, nil),
		memUsageDesc: prometheus.NewDesc("ikuai_memory_usage_bytes", "",
			[]string{}, nil),
		memCachedDesc: prometheus.NewDesc("ikuai_memory_cached_bytes", "",
			[]string{}, nil),
		memBuffersDesc: prometheus.NewDesc("ikuai_memory_buffers_bytes", "",
			[]string{}, nil),
		lanDeviceDesc: prometheus.NewDesc("ikuai_device_info", "ikuai_device_info",
			[]string{"id", "mac", "hostname", "ip_addr", "comment", "ip_version"}, nil),
		lanDeviceCountDesc: prometheus.NewDesc("ikuai_device_count", "",
			[]string{}, nil),
		ifaceInfoDesc: prometheus.NewDesc("ikuai_iface_info", "",
			[]string{"id", "interface", "comment", "internet", "parent_interface", "ip_addr"}, nil),
		UpDesc: prometheus.NewDesc("ikuai_up", "",
			[]string{"id"}, nil),
		UpTimeDesc: prometheus.NewDesc("ikuai_uptime", "",
			[]string{"id"}, nil),
		streamUpBytesDesc: prometheus.NewDesc("ikuai_network_send_bytes", "",
			[]string{"id"}, nil),
		streamDownBytesDesc: prometheus.NewDesc("ikuai_network_recv_bytes", "",
			[]string{"id"}, nil),
		streamUpSpeedDesc: prometheus.NewDesc("ikuai_network_send_kbytes_per_second", "",
			[]string{"id"}, nil),
		streamDownSpeedDesc: prometheus.NewDesc("ikuai_network_recv_kbytes_per_second", "",
			[]string{"id"}, nil),
		connCountDesc: prometheus.NewDesc("ikuai_network_conn_count", "",
			[]string{"id"}, nil),
		dnatInfoDesc: prometheus.NewDesc("ikuai_dnat_info", "iKuai DNAT/port-mapping rule",
			[]string{"id", "tagname", "comment", "interface", "protocol", "wan_port", "lan_addr", "lan_port", "enabled"}, nil),
		dnatTotalDesc: prometheus.NewDesc("ikuai_dnat_total", "Total DNAT rules",
			nil, nil),
		dnatEnabledTotalDesc: prometheus.NewDesc("ikuai_dnat_enabled_total", "Enabled DNAT rules",
			nil, nil),
		dnatDisabledTotalDesc: prometheus.NewDesc("ikuai_dnat_disabled_total", "Disabled DNAT rules",
			nil, nil),
		dnatConnectionsDesc: prometheus.NewDesc("ikuai_dnat_connections", "Current connections attributed to a DNAT rule",
			[]string{"tagname", "interface", "protocol", "wan_port", "lan_addr", "lan_port"}, nil),
		sessionTotalDesc: prometheus.NewDesc("ikuai_session_total", "Current connection sessions reported by iKuai",
			nil, nil),
		sessionInfoDesc: prometheus.NewDesc("ikuai_session_info", "Optional per-session detail (high cardinality)",
			[]string{"protocol", "src_addr", "src_port", "dst_addr", "dst_port", "terminal_addr", "app_name"}, nil),
		MetricErrorDesc: prometheus.NewDesc("ikuai_exporter_metrics_collector_status", "",
			[]string{"type"}, nil),
	}
}

func (i *IKuaiExporter) Describe(descs chan<- *prometheus.Desc) {
	descs <- i.versionDesc
	descs <- i.cpuUsageRatioDesc
	descs <- i.cpuTempDesc
	descs <- i.memSizeDesc
	descs <- i.memUsageDesc
	descs <- i.memCachedDesc
	descs <- i.memBuffersDesc
	descs <- i.lanDeviceDesc
	descs <- i.lanDeviceCountDesc
	descs <- i.ifaceInfoDesc
	descs <- i.UpDesc
	descs <- i.UpTimeDesc
	descs <- i.streamUpBytesDesc
	descs <- i.streamDownBytesDesc
	descs <- i.streamUpSpeedDesc
	descs <- i.streamDownSpeedDesc
	descs <- i.connCountDesc
	descs <- i.dnatInfoDesc
	descs <- i.dnatTotalDesc
	descs <- i.dnatEnabledTotalDesc
	descs <- i.dnatDisabledTotalDesc
	descs <- i.dnatConnectionsDesc
	descs <- i.sessionTotalDesc
	descs <- i.sessionInfoDesc
	descs <- i.MetricErrorDesc
}

func (i *IKuaiExporter) CollectSysStat(metrics chan<- prometheus.Metric) error {
	sysStat, err := i.source.SysStat()
	if err != nil {
		logrus.WithError(err).Error("failed to collect ikuai sysStat")
		return &CollectError{Err: err, Type: "sysStat"}
	}

	metrics <- prometheus.MustNewConstMetric(i.versionDesc, prometheus.GaugeValue, 1,
		sysStat.Version, sysStat.Arch, sysStat.Verstring)

	if len(sysStat.CPUTemp) > 0 {
		metrics <- prometheus.MustNewConstMetric(i.cpuTempDesc, prometheus.GaugeValue, float64(sysStat.CPUTemp[0]))
	}

	for idx, item := range sysStat.CPU {
		s := strings.TrimSuffix(item, "%")
		per, _ := strconv.ParseFloat(s, 64)
		metrics <- prometheus.MustNewConstMetric(i.cpuUsageRatioDesc, prometheus.GaugeValue, per/100,
			fmt.Sprintf("core/%v", idx))
	}

	metrics <- prometheus.MustNewConstMetric(i.memSizeDesc, prometheus.GaugeValue, float64(sysStat.MemTotal))
	metrics <- prometheus.MustNewConstMetric(i.memUsageDesc, prometheus.GaugeValue,
		float64(sysStat.MemTotal-sysStat.MemAvail))
	metrics <- prometheus.MustNewConstMetric(i.memCachedDesc, prometheus.GaugeValue, float64(sysStat.MemCached))
	metrics <- prometheus.MustNewConstMetric(i.memBuffersDesc, prometheus.GaugeValue, float64(sysStat.MemBuffers))
	metrics <- prometheus.MustNewConstMetric(i.lanDeviceCountDesc, prometheus.GaugeValue, float64(sysStat.OnlineUser))

	metrics <- prometheus.MustNewConstMetric(i.UpTimeDesc, prometheus.CounterValue, float64(sysStat.Uptime), "host")
	metrics <- prometheus.MustNewConstMetric(i.streamUpBytesDesc, prometheus.CounterValue, float64(sysStat.StreamUp), "host")
	metrics <- prometheus.MustNewConstMetric(i.streamDownBytesDesc, prometheus.CounterValue, float64(sysStat.StreamDown), "host")
	metrics <- prometheus.MustNewConstMetric(i.streamUpSpeedDesc, prometheus.GaugeValue, sysStat.Upload, "host")
	metrics <- prometheus.MustNewConstMetric(i.streamDownSpeedDesc, prometheus.GaugeValue, sysStat.Download, "host")
	metrics <- prometheus.MustNewConstMetric(i.connCountDesc, prometheus.GaugeValue, sysStat.ConnectNum, "host")
	return nil
}

func (i *IKuaiExporter) CollectLanDevices(metrics chan<- prometheus.Metric) error {
	devices, err := i.source.LanDevices()
	if err != nil {
		logrus.WithError(err).Error("failed to collect ikuai lanDevice")
		return &CollectError{Err: err, Type: "lanDevice"}
	}

	for _, device := range devices {
		deviceId := fmt.Sprintf("device/%v", device.IPAddr)
		ipVer := "4"
		if strings.ContainsAny(device.IPAddr, ":") {
			ipVer = "6"
		}

		metrics <- prometheus.MustNewConstMetric(i.lanDeviceDesc, prometheus.GaugeValue, 1,
			deviceId, device.MAC, device.Hostname, device.IPAddr, device.Comment, ipVer)
		metrics <- prometheus.MustNewConstMetric(i.streamUpBytesDesc, prometheus.CounterValue, device.TotalUp, deviceId)
		metrics <- prometheus.MustNewConstMetric(i.streamDownBytesDesc, prometheus.CounterValue, device.TotalDown, deviceId)
		metrics <- prometheus.MustNewConstMetric(i.streamUpSpeedDesc, prometheus.GaugeValue, device.Upload, deviceId)
		metrics <- prometheus.MustNewConstMetric(i.streamDownSpeedDesc, prometheus.GaugeValue, device.Download, deviceId)
		metrics <- prometheus.MustNewConstMetric(i.connCountDesc, prometheus.GaugeValue, device.ConnectNum, deviceId)
	}
	return nil
}

func (i *IKuaiExporter) CollectInterfaceInfo(metrics chan<- prometheus.Metric) error {
	ifaces, err := i.source.Interfaces()
	if err != nil {
		logrus.WithError(err).Error("failed to collect ikuai interfaceInfo")
		return &CollectError{Err: err, Type: "interfaceInfo"}
	}

	for _, iface := range ifaces {
		ifaceId := fmt.Sprintf("iface/%v", iface.Interface)
		up := 0.0
		if iface.Up {
			up = 1
		}
		metrics <- prometheus.MustNewConstMetric(i.ifaceInfoDesc, prometheus.GaugeValue, 1,
			ifaceId, iface.Interface, iface.Comment, iface.Internet, iface.ParentInterface, iface.IPAddr)
		metrics <- prometheus.MustNewConstMetric(i.UpDesc, prometheus.GaugeValue, up, ifaceId)
		metrics <- prometheus.MustNewConstMetric(i.UpTimeDesc, prometheus.CounterValue, float64(iface.Uptime), ifaceId)
		metrics <- prometheus.MustNewConstMetric(i.streamUpBytesDesc, prometheus.CounterValue, iface.TotalUp, ifaceId)
		metrics <- prometheus.MustNewConstMetric(i.streamDownBytesDesc, prometheus.CounterValue, iface.TotalDown, ifaceId)
		metrics <- prometheus.MustNewConstMetric(i.streamUpSpeedDesc, prometheus.GaugeValue, iface.Upload, ifaceId)
		metrics <- prometheus.MustNewConstMetric(i.streamDownSpeedDesc, prometheus.GaugeValue, iface.Download, ifaceId)
		metrics <- prometheus.MustNewConstMetric(i.connCountDesc, prometheus.GaugeValue, iface.ConnectNum, ifaceId)
	}
	return nil
}

// scrapeSessionCache avoids calling collect_conn twice when both dnat and session modules run.
type scrapeSessionCache struct {
	loaded   bool
	sessions []Session
	err      error
}

func (i *IKuaiExporter) loadSessions(cache *scrapeSessionCache) ([]Session, error) {
	if cache.loaded {
		return cache.sessions, cache.err
	}
	cache.loaded = true

	sessions, err := i.source.Sessions()
	if err != nil {
		if errors.Is(err, ErrSessionUnsupported) {
			logrus.Warn("session API not available on this iKuai version")
		} else {
			logrus.WithError(err).Error("failed to collect ikuai sessions")
		}
		cache.err = &CollectError{Err: err, Type: "session"}
		return nil, cache.err
	}
	cache.sessions = sessions
	return cache.sessions, nil
}

func (i *IKuaiExporter) CollectDNAT(metrics chan<- prometheus.Metric, cache *scrapeSessionCache) error {
	rules, err := i.source.DNAT()
	if err != nil {
		logrus.WithError(err).Error("failed to collect ikuai dnat rules")
		return &CollectError{Err: err, Type: "dnat"}
	}

	sessions, sErr := i.loadSessions(cache)
	if sErr != nil {
		logrus.WithError(sErr).Warn("session data unavailable while collecting dnat")
		sessions = nil
	}

	counts := CountDNATConnections(rules, sessions)
	enabled := 0
	for _, rule := range rules {
		state := "no"
		if rule.Enabled {
			state = "yes"
			enabled++
		}

		metrics <- prometheus.MustNewConstMetric(i.dnatInfoDesc, prometheus.GaugeValue, 1,
			strconv.FormatInt(rule.ID, 10),
			rule.Tagname,
			rule.Comment,
			rule.Interface,
			rule.Protocol,
			rule.WANPort,
			rule.LANAddr,
			rule.LANPort,
			state,
		)

		if rule.Enabled {
			metrics <- prometheus.MustNewConstMetric(i.dnatConnectionsDesc, prometheus.GaugeValue,
				float64(counts[rule.ID]),
				rule.Tagname,
				rule.Interface,
				rule.Protocol,
				rule.WANPort,
				rule.LANAddr,
				rule.LANPort,
			)
		}
	}

	metrics <- prometheus.MustNewConstMetric(i.dnatTotalDesc, prometheus.GaugeValue, float64(len(rules)))
	metrics <- prometheus.MustNewConstMetric(i.dnatEnabledTotalDesc, prometheus.GaugeValue, float64(enabled))
	metrics <- prometheus.MustNewConstMetric(i.dnatDisabledTotalDesc, prometheus.GaugeValue, float64(len(rules)-enabled))
	return nil
}

func (i *IKuaiExporter) CollectSession(metrics chan<- prometheus.Metric, cache *scrapeSessionCache) error {
	sessions, err := i.loadSessions(cache)
	if err != nil {
		return err
	}

	metrics <- prometheus.MustNewConstMetric(i.sessionTotalDesc, prometheus.GaugeValue, float64(len(sessions)))

	if !i.sessionDetail {
		return nil
	}

	limit := i.sessionDetailLimit
	if len(sessions) > limit {
		logrus.WithFields(logrus.Fields{
			"total": len(sessions),
			"limit": limit,
		}).Warn("session detail truncated to limit")
		sessions = truncateSessions(sessions, limit)
	}

	for _, s := range sessions {
		metrics <- prometheus.MustNewConstMetric(i.sessionInfoDesc, prometheus.GaugeValue, 1,
			s.Protocol,
			s.SrcAddr,
			s.SrcPort,
			s.DSTAddr,
			s.DSTPort,
			s.TerminalAddr,
			s.AppName,
		)
	}
	return nil
}

func (i *IKuaiExporter) Collect(metrics chan<- prometheus.Metric) {
	defer func() {
		if errRecover := recover(); errRecover != nil {
			logrus.WithField("err", errRecover).Error("collect ikuai panic")

			metrics <- prometheus.MustNewConstMetric(i.UpDesc, prometheus.GaugeValue, 0,
				"host")
		}
	}()

	errCounter := 0
	cache := &scrapeSessionCache{}

	for _, t := range i.modules {
		var cErr error

		switch t {
		case "sysStat":
			cErr = i.CollectSysStat(metrics)
		case "lanDevice":
			cErr = i.CollectLanDevices(metrics)
		case "interfaceInfo":
			cErr = i.CollectInterfaceInfo(metrics)
		case "dnat":
			cErr = i.CollectDNAT(metrics, cache)
		case "session":
			cErr = i.CollectSession(metrics, cache)
		}

		errStatus := 0
		if cErr != nil {
			errStatus = 1
			errCounter++
		}

		metrics <- prometheus.MustNewConstMetric(i.MetricErrorDesc, prometheus.GaugeValue, float64(errStatus), t)
	}

	// 所有类型都采集失败才标定 host 为 down
	if errCounter == len(i.modules) {
		metrics <- prometheus.MustNewConstMetric(i.UpDesc, prometheus.GaugeValue, 0, "host")
		return
	}

	metrics <- prometheus.MustNewConstMetric(i.UpDesc, prometheus.GaugeValue, 1, "host")
}
