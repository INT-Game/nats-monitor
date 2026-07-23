package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"
)

type config struct {
	listenAddr string
	monitorURL string
	targets    []monitorTarget
	interval   time.Duration
	auth       authConfig
}

type monitorTarget struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type fileConfig struct {
	ListenAddr         string          `json:"listen_addr"`
	NATSMonitorURL     string          `json:"nats_monitor_url"`
	NATSMonitorTargets []monitorTarget `json:"nats_monitor_targets"`
	RefreshInterval    string          `json:"refresh_interval"`
	Auth               struct {
		Username     string `json:"username"`
		Password     string `json:"password"`
		SessionTTL   string `json:"session_ttl"`
		SecureCookie bool   `json:"secure_cookie"`
	} `json:"auth"`
}

func loadConfig(path string) (config, error) {
	values := fileConfig{
		ListenAddr:      "0.0.0.0:18222",
		NATSMonitorURL:  "http://127.0.0.1:8222",
		RefreshInterval: "5s",
	}
	values.Auth.Username = "admin"
	values.Auth.Password = "CHANGE_ME"
	values.Auth.SessionTTL = "12h"
	data, err := os.ReadFile(path)
	if err == nil {
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&values); err != nil {
			return config{}, fmt.Errorf("read config %s: %w", path, err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return config{}, fmt.Errorf("read config %s: %w", path, err)
	}

	values.ListenAddr = envOr("LISTEN_ADDR", values.ListenAddr)
	values.NATSMonitorURL = envOr("NATS_MONITOR_URL", values.NATSMonitorURL)
	values.RefreshInterval = envOr("REFRESH_INTERVAL", values.RefreshInterval)
	values.Auth.Username = envOr("MONITOR_USERNAME", values.Auth.Username)
	values.Auth.Password = envOr("MONITOR_PASSWORD", values.Auth.Password)
	interval, err := time.ParseDuration(values.RefreshInterval)
	if err != nil || interval < time.Second {
		return config{}, errors.New("refresh_interval must be a duration of at least 1s")
	}
	if values.ListenAddr == "" || values.NATSMonitorURL == "" {
		return config{}, errors.New("listen_addr and nats_monitor_url are required")
	}
	targets := make([]monitorTarget, 0, len(values.NATSMonitorTargets))
	names := make(map[string]struct{}, len(values.NATSMonitorTargets))
	for index, target := range values.NATSMonitorTargets {
		target.Name = strings.TrimSpace(target.Name)
		if target.Name == "" || strings.TrimSpace(target.URL) == "" {
			return config{}, fmt.Errorf("nats_monitor_targets[%d] requires name and url", index)
		}
		if _, exists := names[target.Name]; exists {
			return config{}, fmt.Errorf("duplicate nats_monitor_targets name %q", target.Name)
		}
		parsed, err := parseMonitorURL(target.URL)
		if err != nil {
			return config{}, fmt.Errorf("nats_monitor_targets[%d]: %w", index, err)
		}
		target.URL = sanitizedURL(parsed)
		targets = append(targets, target)
		names[target.Name] = struct{}{}
	}
	sessionTTL, err := time.ParseDuration(values.Auth.SessionTTL)
	if err != nil || sessionTTL < 5*time.Minute {
		return config{}, errors.New("auth.session_ttl must be a duration of at least 5m")
	}
	if values.Auth.Username == "" || values.Auth.Password == "" {
		return config{}, errors.New("auth.username and auth.password are required")
	}
	if values.Auth.Password == "CHANGE_ME" {
		return config{}, errors.New("auth.password must be changed from CHANGE_ME")
	}

	return config{
		listenAddr: values.ListenAddr,
		monitorURL: values.NATSMonitorURL,
		targets:    targets,
		interval:   interval,
		auth: authConfig{
			username:     values.Auth.Username,
			password:     values.Auth.Password,
			sessionTTL:   sessionTTL,
			secureCookie: values.Auth.SecureCookie,
		},
	}, nil
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func sanitizedURL(value *url.URL) string {
	copy := *value
	copy.User = nil
	return copy.String()
}
