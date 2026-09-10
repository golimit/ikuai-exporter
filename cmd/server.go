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

		timeout := time.Duration(opts.Timeout) * time.Second
		src, err := pkg.NewSource(strings.TrimSpace(opts.URL), opts.Username, opts.Password, opts.InsecureSkip, timeout)
		if err != nil {
			return err
		}

		registry := prometheus.NewRegistry()
		registry.MustRegister(pkg.NewIKuaiExporter(src, opts.Modules, opts.SessionDetail, opts.SessionDetailLimit))

		http.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{Registry: registry}))

		logrus.Infof("iKuai exporter %v started on :9090 (api major=%d)", version.Version(), src.Major())
		logrus.Fatal(http.ListenAndServe(":9090", nil))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(serverCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// serverCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// serverCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	serverCmd.Flags().StringVar(&opts.URL, "url", opts.URL, "iKuai URL")
	serverCmd.Flags().StringVarP(&opts.Username, "username", "u", opts.Username, "iKuai username")
	serverCmd.Flags().StringVarP(&opts.Password, "password", "p", opts.Password, "The password for the user on iKuai")
	serverCmd.Flags().BoolVar(&opts.InsecureSkip, "insecure-skip", opts.InsecureSkip, "Skip iKuai certificate verification")
	serverCmd.Flags().IntVar(&opts.Timeout, "timeout", opts.Timeout, "The timeout (seconds) for a request to iKuai API. ")
	serverCmd.Flags().StringSliceVar(&opts.Modules, "modules", opts.Modules, "The modules to be collected.")
	serverCmd.Flags().BoolVar(&opts.SessionDetail, "session-detail", opts.SessionDetail, "Export per-session detail metrics (high cardinality)")
	serverCmd.Flags().IntVar(&opts.SessionDetailLimit, "session-detail-limit", opts.SessionDetailLimit, "Max number of session detail series")
	serverCmd.Flags().StringVarP(&opts.Level, "level", "l", opts.Level, "Log level")

	viper.BindEnv("url", "IK_URL")
	viper.BindEnv("username", "IK_USER")
	viper.BindEnv("password", "IK_PWD")
	viper.BindEnv("session-detail", "IKUAI_SESSION_DETAIL")
	viper.BindEnv("session-detail-limit", "IKUAI_SESSION_DETAIL_LIMIT")
}
