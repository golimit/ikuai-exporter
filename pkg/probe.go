package pkg

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

// NewProbeHandler serves GET /probe?target=<ikuai_url>&auth_module=<name>.
func NewProbeHandler(cfg *ExporterConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "only GET is allowed", http.StatusMethodNotAllowed)
			return
		}

		rawTarget := strings.TrimSpace(r.URL.Query().Get("target"))
		if rawTarget == "" {
			http.Error(w, "query parameter 'target' is required", http.StatusBadRequest)
			return
		}

		targetURL, err := normalizeTargetURL(rawTarget)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if err := validateTargetHost(targetURL.Hostname(), cfg.AllowedCIDRs); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		authModule := strings.TrimSpace(r.URL.Query().Get("auth_module"))
		auth, err := cfg.Auth(authModule)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		timeout := time.Duration(cfg.Timeout) * time.Second
		src, err := NewSource(targetURL.String(), auth.Username, auth.Password, cfg.InsecureSkipTLS, timeout)
		if err != nil {
			logrus.WithError(err).WithField("target", targetURL.Redacted()).Warn("probe: failed to create source")
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}

		registry := prometheus.NewRegistry()
		registry.MustRegister(NewIKuaiExporter(
			src,
			cfg.Modules,
			cfg.SessionDetail,
			cfg.SessionDetailLimit,
			cfg.DNATSessionDetail,
			cfg.DNATSessionDetailLimit,
		))

		h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{Registry: registry})
		h.ServeHTTP(w, r)
	}
}

func normalizeTargetURL(raw string) (*url.URL, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil, fmt.Errorf("target is empty")
	}
	if !strings.Contains(s, "://") {
		s = "http://" + s
	}
	u, err := url.Parse(s)
	if err != nil {
		return nil, fmt.Errorf("invalid target URL: %w", err)
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return nil, fmt.Errorf("target scheme must be http or https")
	}
	if strings.TrimSpace(u.Host) == "" {
		return nil, fmt.Errorf("target host is required")
	}
	u.Path = strings.TrimSuffix(u.Path, "/")
	if u.Path == "" {
		u.Path = ""
	}
	u.RawQuery = ""
	u.Fragment = ""
	return u, nil
}

func validateTargetHost(host string, allowedCIDRs []string) error {
	if len(allowedCIDRs) == 0 {
		return nil
	}
	host = strings.Trim(host, "[]")
	ip := net.ParseIP(host)
	if ip == nil {
		ips, err := net.LookupIP(host)
		if err != nil || len(ips) == 0 {
			return fmt.Errorf("target host %q could not be resolved", host)
		}
		ip = ips[0]
	}
	for _, cidr := range allowedCIDRs {
		_, network, err := net.ParseCIDR(strings.TrimSpace(cidr))
		if err != nil {
			continue
		}
		if network.Contains(ip) {
			return nil
		}
	}
	return fmt.Errorf("target host %q is not in allowed_cidrs", host)
}
