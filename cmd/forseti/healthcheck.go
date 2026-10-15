package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

const defaultMetricsPort = 9099

// runHealthcheck probes the running daemon's /healthz endpoint. It never
// contacts Pi-hole targets and never fails because the config is invalid:
// the config is only read for metrics.port.
func runHealthcheck(args []string) int {
	fs := flag.NewFlagSet("healthcheck", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	cfgPath := fs.String("config", "", "path to forseti config file (used for metrics.port)")
	addr := fs.String("addr", "", "host:port of the metrics server (overrides --config)")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	target := *addr
	if target == "" {
		target = fmt.Sprintf("127.0.0.1:%d", metricsPort(*cfgPath))
	}
	return probeHealthz("http://" + target + "/healthz")
}

func probeHealthz(url string) int {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unhealthy: %v\n", err)
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		fmt.Fprintf(os.Stderr, "unhealthy: %s %s\n", resp.Status, body)
		return 1
	}
	fmt.Println("ok")
	return 0
}

// metricsPort reads metrics.port from the config file, tolerating invalid or
// missing files by falling back to the default port.
func metricsPort(path string) int {
	if path == "" {
		return defaultMetricsPort
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return defaultMetricsPort
	}
	var c struct {
		Metrics struct {
			Port int `yaml:"port"`
		} `yaml:"metrics"`
	}
	if err := yaml.Unmarshal([]byte(os.ExpandEnv(string(data))), &c); err != nil || c.Metrics.Port <= 0 {
		return defaultMetricsPort
	}
	return c.Metrics.Port
}
