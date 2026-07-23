package main

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCalculateRates(t *testing.T) {
	start := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	got := calculateRates(
		varzResponse{InMsgs: 100, OutMsgs: 50, InBytes: 1000, OutBytes: 500},
		varzResponse{InMsgs: 160, OutMsgs: 90, InBytes: 1800, OutBytes: 700},
		start,
		start.Add(10*time.Second),
	)
	if math.Abs(got.MessagesPerSecond-10) > 0.001 {
		t.Fatalf("messages/s = %v, want 10", got.MessagesPerSecond)
	}
	if math.Abs(got.BytesPerSecond-100) > 0.001 {
		t.Fatalf("bytes/s = %v, want 100", got.BytesPerSecond)
	}
}

func TestLoadConfig(t *testing.T) {
	t.Setenv("LISTEN_ADDR", "")
	t.Setenv("NATS_MONITOR_URL", "")
	t.Setenv("REFRESH_INTERVAL", "")
	path := filepath.Join(t.TempDir(), "config.json")
	err := os.WriteFile(path, []byte(`{
  "listen_addr": "0.0.0.0:19000",
  "nats_monitor_url": "http://nats:8222",
  "nats_monitor_targets": [
    {"name": "Local", "url": "127.0.0.1"},
    {"name": "Public", "url": "https://nats.example.com:8222"}
  ],
  "refresh_interval": "10s",
  "auth": {
    "username": "ops",
    "password": "secret-value",
    "session_ttl": "24h",
    "secure_cookie": true
  }
}`), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	got, err := loadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.listenAddr != "0.0.0.0:19000" || got.monitorURL != "http://nats:8222" || got.interval != 10*time.Second {
		t.Fatalf("unexpected config: %+v", got)
	}
	if got.auth.username != "ops" || got.auth.password != "secret-value" || got.auth.sessionTTL != 24*time.Hour || !got.auth.secureCookie {
		t.Fatalf("unexpected auth config: %+v", got.auth)
	}
	if len(got.targets) != 2 || got.targets[0].URL != "http://127.0.0.1:8222" || got.targets[1].Name != "Public" {
		t.Fatalf("unexpected monitor targets: %+v", got.targets)
	}
}

func TestLoadConfigRejectsDefaultPassword(t *testing.T) {
	t.Setenv("MONITOR_USERNAME", "")
	t.Setenv("MONITOR_PASSWORD", "")
	_, err := loadConfig(filepath.Join(t.TempDir(), "missing.json"))
	if err == nil {
		t.Fatal("default password was accepted")
	}
}

func TestAuthenticatorSessionAndCredentials(t *testing.T) {
	auth, err := newAuthenticator(authConfig{username: "admin", password: "correct", sessionTTL: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	if !auth.credentialsValid("admin", "correct") || auth.credentialsValid("admin", "wrong") {
		t.Fatal("credential validation returned an unexpected result")
	}

	response := httptest.NewRecorder()
	now := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	auth.setSession(response, now)
	request := httptest.NewRequest("GET", "/", nil)
	request.AddCookie(response.Result().Cookies()[0])
	if !auth.isAuthenticated(request, now.Add(30*time.Minute)) {
		t.Fatal("valid session was rejected")
	}
	if auth.isAuthenticated(request, now.Add(2*time.Hour)) {
		t.Fatal("expired session was accepted")
	}

	cookie := response.Result().Cookies()[0]
	cookie.Value += "tampered"
	tampered := httptest.NewRequest("GET", "/", nil)
	tampered.AddCookie(cookie)
	if auth.isAuthenticated(tampered, now) {
		t.Fatal("tampered session was accepted")
	}
}

func TestLoginRateLimit(t *testing.T) {
	auth, err := newAuthenticator(authConfig{username: "admin", password: "correct", sessionTTL: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	for range 5 {
		auth.recordFailure("192.0.2.10:1234", now)
	}
	if auth.allowLogin("192.0.2.10:9876", now.Add(time.Minute)) {
		t.Fatal("rate-limited address was allowed")
	}
	if !auth.allowLogin("192.0.2.11:1234", now.Add(time.Minute)) {
		t.Fatal("unrelated address was blocked")
	}
}

func TestCollectorPaginatesConnectionsAndSubscriptions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		offset := r.URL.Query().Get("offset")
		switch r.URL.Path {
		case "/connz":
			if r.URL.Query().Get("auth") != "true" {
				t.Error("connz request did not include auth=true")
			}
			page := connzResponse{Total: 3}
			if offset == "0" {
				page.Connections = []connection{{CID: 1}, {CID: 2}}
			} else {
				page.Connections = []connection{{CID: 3}}
			}
			_ = json.NewEncoder(w).Encode(page)
		case "/subsz":
			page := subszResponse{Total: 3}
			if offset == "0" {
				page.List = []subscriptionDetail{{Subject: "one"}, {Subject: "two"}}
			} else {
				page.List = []subscriptionDetail{{Subject: "three"}}
			}
			_ = json.NewEncoder(w).Encode(page)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	collector, err := newCollector(server.URL, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	connections, err := collector.getAllConnections(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	subscriptions, err := collector.getAllSubscriptions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(connections.Connections) != 3 || connections.Connections[2].CID != 3 {
		t.Fatalf("unexpected connections: %+v", connections.Connections)
	}
	if len(subscriptions.List) != 3 || subscriptions.List[2].Subject != "three" {
		t.Fatalf("unexpected subscriptions: %+v", subscriptions.List)
	}
}

func TestRecentClosedSlowConsumers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/connz" || r.URL.Query().Get("state") != "closed" || r.URL.Query().Get("auth") != "true" {
			t.Errorf("unexpected request: %s", r.URL.String())
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(connzResponse{
			Total: 3,
			Connections: []connection{
				{CID: 1, Reason: "Client Closed"},
				{CID: 2, Reason: "Slow Consumer"},
				{CID: 3, Reason: "Slow Consumer (Write Deadline)"},
			},
		})
	}))
	defer server.Close()

	collector, err := newCollector(server.URL, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	closed, err := collector.getRecentClosedConnections(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	slow := filterSlowConsumers(closed.Connections)
	if len(slow) != 2 || slow[0].CID != 2 || slow[1].CID != 3 {
		t.Fatalf("unexpected slow consumers: %+v", slow)
	}
}

func TestParseMonitorURL(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    string
		wantErr bool
	}{
		{name: "private", value: "10.1.2.3:8222", want: "http://10.1.2.3:8222"},
		{name: "default port", value: "http://192.168.1.20", want: "http://192.168.1.20:8222"},
		{name: "localhost", value: "localhost:8222", want: "http://localhost:8222"},
		{name: "public", value: "http://8.8.8.8:8222", want: "http://8.8.8.8:8222"},
		{name: "domain", value: "http://nats.example.com:8222", want: "http://nats.example.com:8222"},
		{name: "path", value: "http://10.1.2.3:8222/varz", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseMonitorURL(test.value)
			if test.wantErr {
				if err == nil {
					t.Fatalf("parseMonitorURL(%q) unexpectedly succeeded", test.value)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.String() != test.want {
				t.Fatalf("target = %q, want %q", got.String(), test.want)
			}
		})
	}
}

func TestCollectorSwitchTarget(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/healthz":
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		case "/varz":
			_ = json.NewEncoder(w).Encode(varzResponse{ServerID: "test-server", ServerName: "test"})
		case "/connz":
			_ = json.NewEncoder(w).Encode(connzResponse{Connections: []connection{}})
		case "/subsz":
			_ = json.NewEncoder(w).Encode(subszResponse{List: []subscriptionDetail{}})
		case "/routez":
			_ = json.NewEncoder(w).Encode(routezResponse{Routes: []route{}})
		case "/gatewayz":
			_ = json.NewEncoder(w).Encode(gatewayzResponse{})
		case "/leafz":
			_ = json.NewEncoder(w).Encode(leafzResponse{})
		case "/accountz":
			_ = json.NewEncoder(w).Encode(accountzResponse{})
		case "/accstatz":
			_ = json.NewEncoder(w).Encode(accountStatzResponse{})
		case "/jsz":
			_ = json.NewEncoder(w).Encode(jetStreamResponse{})
		default:
			http.NotFound(w, r)
		}
	}))
	defer target.Close()

	collector, err := newCollector("http://127.0.0.1:1", 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	got, err := collector.switchTarget(context.Background(), target.URL)
	if err != nil {
		t.Fatal(err)
	}
	if got != target.URL {
		t.Fatalf("target = %q, want %q", got, target.URL)
	}
	snapshot := collector.snapshot()
	if !snapshot.Healthy || snapshot.Server.ServerID != "test-server" {
		t.Fatalf("unexpected snapshot after switch: %+v", snapshot)
	}
}
