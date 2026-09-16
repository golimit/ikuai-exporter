/*
Copyright © 2026 Jakes Lee
*/
package cmd

import (
	"net/http"
	"strings"
	"time"

	"github.com/jakeslee/ikuai-exporter/cmd/options"
	"github.com/jakeslee/ikuai-exporter/pkg"
	"github.com/jakeslee/ikuai-exporter/pkg/version"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var opts = options.NewServerOptions()
var configFile string
var webListenAddress string

// serverCmd represents the server command
var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Run metrics endpoint",
	RunE: func(cmd *cobra.Command, args []string) error {
		level, err := logrus.ParseLevel(opts.Level)
		if err != nil {
			return err
		}
		logrus.SetLevel(level)

		mux := http.NewServeMux()

		if strings.TrimSpace(configFile) != "" {
			cfg, err := pkg.LoadExporterConfig(configFile)
			if err != nil {
				return err
			}
			mux.HandleFunc("/probe", pkg.NewProbeHandler(cfg))
			mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("ok"))
			})
			logrus.Infof("iKuai exporter %v started on %s (multi-target /probe)", version.Version(), webListenAddress)
			logrus.Fatal(http.ListenAndServe(webListenAddress, mux))
			return nil
		}

		timeout := time.Duration(opts.Timeout) * time.Second
		src, err := pkg.NewSource(strings.TrimSpace(opts.URL), opts.Username, opts.Password, opts.InsecureSkip, timeout)
		if err != nil {
			return err
		}

		registry := prometheus.NewRegistry()
		registry.MustRegister(pkg.NewIKuaiExporter(src, opts.Modules, opts.SessionDetail, opts.SessionDetailLimit, opts.DNATSessionDetail, opts.DNATSessionDetailLimit))

		mux.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{Registry: registry}))

		logrus.Infof("iKuai exporter %v started on %s (api major=%d)", version.Version(), webListenAddress, src.Major())
		logrus.Fatal(http.ListenAndServe(webListenAddress, mux))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(serverCmd)

	serverCmd.Flags().StringVar(&opts.URL, "url", opts.URL, "iKuai URL (legacy single-target mode)")
	serverCmd.Flags().StringVarP(&opts.Username, "username", "u", opts.Username, "iKuai username")
	serverCmd.Flags().StringVarP(&opts.Password, "password", "p", opts.Password, "The password for the user on iKuai")
	serverCmd.Flags().BoolVar(&opts.InsecureSkip, "insecure-skip", opts.InsecureSkip, "Skip iKuai certificate verification")
	serverCmd.Flags().IntVar(&opts.Timeout, "timeout", opts.Timeout, "The timeout (seconds) for a request to iKuai API. ")
	serverCmd.Flags().StringSliceVar(&opts.Modules, "modules", opts.Modules, "The modules to be collected.")
	serverCmd.Flags().BoolVar(&opts.SessionDetail, "session-detail", opts.SessionDetail, "Export per-session detail metrics (high cardinality)")
	serverCmd.Flags().IntVar(&opts.SessionDetailLimit, "session-detail-limit", opts.SessionDetailLimit, "Max number of session detail series")
	serverCmd.Flags().BoolVar(&opts.DNATSessionDetail, "dnat-session-detail", opts.DNATSessionDetail, "Export per-connection DNAT inbound session detail metrics")
	serverCmd.Flags().IntVar(&opts.DNATSessionDetailLimit, "dnat-session-detail-limit", opts.DNATSessionDetailLimit, "Max number of DNAT session detail series")
	serverCmd.Flags().StringVarP(&opts.Level, "level", "l", opts.Level, "Log level")
	serverCmd.Flags().StringVar(&configFile, "config.file", "", "YAML config for multi-target /probe mode")
	serverCmd.Flags().StringVar(&webListenAddress, "web.listen-address", ":9401", "HTTP listen address")

	viper.BindEnv("url", "IKUAI_URL", "IK_URL")
	viper.BindEnv("username", "IKUAI_USERNAME", "IK_USER")
	viper.BindEnv("password", "IKUAI_PASSWORD", "IK_PWD")
	viper.BindEnv("config.file", "IKUAI_CONFIG_FILE")
	viper.BindEnv("web.listen-address", "IKUAI_WEB_LISTEN_ADDRESS")
	viper.BindEnv("session-detail", "IKUAI_SESSION_DETAIL")
	viper.BindEnv("session-detail-limit", "IKUAI_SESSION_DETAIL_LIMIT")
	viper.BindEnv("dnat-session-detail", "IKUAI_DNAT_SESSION_DETAIL")
	viper.BindEnv("dnat-session-detail-limit", "IKUAI_DNAT_SESSION_DETAIL_LIMIT")
}
