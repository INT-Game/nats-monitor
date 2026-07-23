package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

type collector struct {
	mu           sync.RWMutex
	collectMu    sync.Mutex
	client       *http.Client
	target       *url.URL
	publicTarget string
	interval     time.Duration
	current      snapshot
	previous     varzResponse
	previousAt   time.Time
	history      []historyPoint
	historyMax   int
}

func newCollector(rawURL string, interval time.Duration) (*collector, error) {
	target, err := parseMonitorURL(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid NATS_MONITOR_URL %q: %w", rawURL, err)
	}
	maxPoints := int(time.Hour / interval)
	if maxPoints < 2 {
		maxPoints = 2
	}
	publicTarget := sanitizedURL(target)
	return &collector{
		client: &http.Client{
			Timeout: 3 * time.Second,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		target:       target,
		publicTarget: publicTarget,
		interval:     interval,
		historyMax:   maxPoints,
		current:      emptySnapshot(publicTarget),
	}, nil
}

func emptySnapshot(target string) snapshot {
	return snapshot{
		Target:              target,
		Warnings:            []string{},
		Connections:         []connection{},
		RecentSlowConsumers: []connection{},
		Routes:              []route{},
		Gateways: gatewayzResponse{
			OutboundGateways: map[string]remoteGateway{},
			InboundGateways:  map[string][]remoteGateway{},
		},
		LeafNodes:    leafzResponse{Leafs: []leafInfo{}},
		Accounts:     accountzResponse{Accounts: []string{}},
		AccountStats: []accountStat{},
		History:      []historyPoint{},
	}
}

func parseMonitorURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errors.New("监控地址不能为空")
	}
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	target, err := url.Parse(raw)
	if err != nil || target.Hostname() == "" {
		return nil, errors.New("监控地址格式无效")
	}
	if target.Scheme != "http" && target.Scheme != "https" {
		return nil, errors.New("监控地址仅支持 http 或 https")
	}
	if target.User != nil || target.RawQuery != "" || target.Fragment != "" {
		return nil, errors.New("监控地址不能包含账号、查询参数或片段")
	}
	if target.Path != "" && target.Path != "/" {
		return nil, errors.New("监控地址不能包含路径")
	}
	if target.Port() == "" {
		target.Host = net.JoinHostPort(target.Hostname(), "8222")
	} else if port, err := strconv.Atoi(target.Port()); err != nil || port < 1 || port > 65535 {
		return nil, errors.New("监控端口无效")
	}
	target.Path = ""
	return target, nil
}

func (c *collector) run(ctx context.Context) {
	c.collect(ctx)
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.collect(ctx)
		}
	}
}

func (c *collector) collect(ctx context.Context) {
	c.collectMu.Lock()
	defer c.collectMu.Unlock()
	c.collectLocked(ctx)
}

func (c *collector) collectLocked(ctx context.Context) {
	attemptAt := time.Now()
	requestCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var server varzResponse
	var connections connzResponse
	var closedConnections connzResponse
	var subscriptions subszResponse
	var routes routezResponse
	var gateways gatewayzResponse
	var leafNodes leafzResponse
	var accounts accountzResponse
	var accountStats accountStatzResponse
	var jetStream jetStreamResponse

	var healthErr, varzErr, connzErr, closedConnzErr, subszErr, routezErr error
	var gatewayzErr, leafzErr, accountzErr, accstatzErr, jszErr error
	var requests sync.WaitGroup
	requests.Add(11)
	go func() { defer requests.Done(); healthErr = c.get(requestCtx, "/healthz", nil) }()
	go func() { defer requests.Done(); varzErr = c.get(requestCtx, "/varz", &server) }()
	go func() { defer requests.Done(); connections, connzErr = c.getAllConnections(requestCtx) }()
	go func() {
		defer requests.Done()
		closedConnections, closedConnzErr = c.getRecentClosedConnections(requestCtx)
	}()
	go func() { defer requests.Done(); subscriptions, subszErr = c.getAllSubscriptions(requestCtx) }()
	go func() { defer requests.Done(); routezErr = c.get(requestCtx, "/routez?subs=detail", &routes) }()
	go func() { defer requests.Done(); gatewayzErr = c.get(requestCtx, "/gatewayz", &gateways) }()
	go func() { defer requests.Done(); leafzErr = c.get(requestCtx, "/leafz?subs=1", &leafNodes) }()
	go func() { defer requests.Done(); accountzErr = c.get(requestCtx, "/accountz", &accounts) }()
	go func() { defer requests.Done(); accstatzErr = c.get(requestCtx, "/accstatz", &accountStats) }()
	go func() { defer requests.Done(); jszErr = c.get(requestCtx, "/jsz", &jetStream) }()
	requests.Wait()

	c.mu.Lock()
	defer c.mu.Unlock()
	c.current.LastAttemptAt = attemptAt
	if healthErr != nil || varzErr != nil {
		c.current.Healthy = false
		c.current.Error = joinErrors(healthErr, varzErr)
		return
	}

	hasPrevious := !c.previousAt.IsZero()
	currentRates := calculateRates(c.previous, server, c.previousAt, attemptAt)
	c.previous = server
	c.previousAt = attemptAt
	if hasPrevious {
		c.history = append(c.history, historyPoint{
			At:          attemptAt,
			MessagesPS:  currentRates.MessagesPerSecond,
			BytesPS:     currentRates.BytesPerSecond,
			Connections: server.Connections,
			CPU:         server.CPU,
			Memory:      server.Mem,
		})
		if len(c.history) > c.historyMax {
			c.history = c.history[len(c.history)-c.historyMax:]
		}
	}

	warnings := make([]string, 0, 10)
	if connzErr != nil {
		warnings = append(warnings, "连接明细暂不可用")
	}
	if closedConnzErr != nil {
		warnings = append(warnings, "最近慢消费者记录暂不可用")
	}
	if subszErr != nil {
		warnings = append(warnings, "订阅统计暂不可用")
	}
	if routezErr != nil {
		warnings = append(warnings, "路由信息暂不可用")
	}
	if gatewayzErr != nil {
		warnings = append(warnings, "Gateway 信息暂不可用")
	}
	if leafzErr != nil {
		warnings = append(warnings, "Leaf Node 信息暂不可用")
	}
	if accountzErr != nil || accstatzErr != nil {
		warnings = append(warnings, "账号统计暂不可用")
	}
	if jszErr != nil {
		warnings = append(warnings, "JetStream 信息暂不可用")
	}

	normalizeSnapshotData(&subscriptions, &gateways, &leafNodes, &accounts, &accountStats)
	c.current = snapshot{
		Target:                   c.publicTarget,
		Healthy:                  true,
		Warnings:                 warnings,
		LastAttemptAt:            attemptAt,
		LastSuccessAt:            attemptAt,
		Server:                   server,
		Rates:                    currentRates,
		Connections:              nonNilConnections(connections.Connections),
		RecentSlowConsumers:      filterSlowConsumers(closedConnections.Connections),
		ClosedConnectionsScanned: len(closedConnections.Connections),
		ClosedConnectionsTotal:   closedConnections.Total,
		Subscriptions:            subscriptions,
		Routes:                   nonNilRoutes(routes.Routes),
		Gateways:                 gateways,
		LeafNodes:                leafNodes,
		Accounts:                 accounts,
		AccountStats:             accountStats.Accounts,
		JetStream:                jetStream,
		History:                  append([]historyPoint(nil), c.history...),
	}
}

func (c *collector) switchTarget(ctx context.Context, raw string) (string, error) {
	target, err := parseMonitorURL(raw)
	if err != nil {
		return "", err
	}
	probeCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	var server varzResponse
	if err := fetch(probeCtx, c.client, target, "/varz", &server); err != nil {
		return "", fmt.Errorf("无法连接该 NATS 监控端点: %w", err)
	}
	if server.ServerID == "" {
		return "", errors.New("目标没有返回有效的 NATS Server 信息")
	}

	c.collectMu.Lock()
	defer c.collectMu.Unlock()
	c.target = target
	c.publicTarget = sanitizedURL(target)
	c.mu.Lock()
	c.previous = varzResponse{}
	c.previousAt = time.Time{}
	c.history = nil
	c.current = emptySnapshot(c.publicTarget)
	c.mu.Unlock()
	c.collectLocked(ctx)
	return c.publicTarget, nil
}

func normalizeSnapshotData(subscriptions *subszResponse, gateways *gatewayzResponse, leafNodes *leafzResponse, accounts *accountzResponse, accountStats *accountStatzResponse) {
	if subscriptions.List == nil {
		subscriptions.List = []subscriptionDetail{}
	}
	if gateways.OutboundGateways == nil {
		gateways.OutboundGateways = map[string]remoteGateway{}
	}
	if gateways.InboundGateways == nil {
		gateways.InboundGateways = map[string][]remoteGateway{}
	}
	if leafNodes.Leafs == nil {
		leafNodes.Leafs = []leafInfo{}
	}
	if accounts.Accounts == nil {
		accounts.Accounts = []string{}
	}
	if accountStats.Accounts == nil {
		accountStats.Accounts = []accountStat{}
	}
}

func (c *collector) getAllConnections(ctx context.Context) (connzResponse, error) {
	const pageSize = 1024
	var result connzResponse
	for offset := 0; ; {
		var page connzResponse
		path := fmt.Sprintf("/connz?auth=true&subs=detail&limit=%d&offset=%d", pageSize, offset)
		if err := c.get(ctx, path, &page); err != nil {
			return connzResponse{}, err
		}
		if offset == 0 {
			result = page
			result.Connections = nil
		}
		result.Connections = append(result.Connections, page.Connections...)
		if len(page.Connections) == 0 || len(result.Connections) >= page.Total {
			break
		}
		offset += len(page.Connections)
	}
	result.Offset = 0
	result.Limit = len(result.Connections)
	return result, nil
}

func (c *collector) getRecentClosedConnections(ctx context.Context) (connzResponse, error) {
	const limit = 1024
	var result connzResponse
	path := fmt.Sprintf("/connz?state=closed&auth=true&sort=stop&limit=%d", limit)
	if err := c.get(ctx, path, &result); err != nil {
		return connzResponse{}, err
	}
	return result, nil
}

func filterSlowConsumers(connections []connection) []connection {
	result := make([]connection, 0)
	for _, item := range connections {
		if strings.Contains(strings.ToLower(item.Reason), "slow consumer") {
			result = append(result, item)
		}
	}
	return result
}

func (c *collector) getAllSubscriptions(ctx context.Context) (subszResponse, error) {
	const pageSize = 1024
	var result subszResponse
	for offset := 0; ; {
		var page subszResponse
		if err := c.get(ctx, fmt.Sprintf("/subsz?subs=1&limit=%d&offset=%d", pageSize, offset), &page); err != nil {
			return subszResponse{}, err
		}
		if offset == 0 {
			result = page
			result.List = nil
		}
		result.List = append(result.List, page.List...)
		if len(page.List) == 0 || len(result.List) >= page.Total {
			break
		}
		offset += len(page.List)
	}
	result.Offset = 0
	result.Limit = len(result.List)
	return result, nil
}

func (c *collector) get(ctx context.Context, path string, dst any) error {
	return fetch(ctx, c.client, c.target, path, dst)
}

func fetch(ctx context.Context, client *http.Client, target *url.URL, path string, dst any) error {
	requestURL := *target
	requestURL.Path = strings.TrimRight(target.Path, "/") + strings.SplitN(path, "?", 2)[0]
	if parts := strings.SplitN(path, "?", 2); len(parts) == 2 {
		requestURL.RawQuery = parts[1]
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.String(), nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s returned %s", path, resp.Status)
	}
	if dst == nil {
		_, err = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
		return err
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 32<<20)).Decode(dst)
}

func (c *collector) snapshot() snapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	copy := c.current
	copy.Warnings = append([]string(nil), c.current.Warnings...)
	copy.Connections = append([]connection(nil), c.current.Connections...)
	copy.RecentSlowConsumers = append([]connection(nil), c.current.RecentSlowConsumers...)
	copy.Routes = append([]route(nil), c.current.Routes...)
	copy.Subscriptions.List = append([]subscriptionDetail(nil), c.current.Subscriptions.List...)
	copy.LeafNodes.Leafs = append([]leafInfo(nil), c.current.LeafNodes.Leafs...)
	copy.Accounts.Accounts = append([]string(nil), c.current.Accounts.Accounts...)
	copy.AccountStats = append([]accountStat(nil), c.current.AccountStats...)
	copy.History = append([]historyPoint(nil), c.current.History...)
	return copy
}

func calculateRates(previous, current varzResponse, previousAt, currentAt time.Time) rates {
	seconds := currentAt.Sub(previousAt).Seconds()
	if previousAt.IsZero() || seconds <= 0 {
		return rates{}
	}
	previousMessages := previous.InMsgs + previous.OutMsgs
	currentMessages := current.InMsgs + current.OutMsgs
	previousBytes := previous.InBytes + previous.OutBytes
	currentBytes := current.InBytes + current.OutBytes
	if currentMessages < previousMessages || currentBytes < previousBytes {
		return rates{}
	}
	return rates{
		MessagesPerSecond: float64(currentMessages-previousMessages) / seconds,
		BytesPerSecond:    float64(currentBytes-previousBytes) / seconds,
	}
}

func joinErrors(errs ...error) string {
	messages := make([]string, 0, len(errs))
	for _, err := range errs {
		if err != nil {
			messages = append(messages, err.Error())
		}
	}
	return strings.Join(messages, "; ")
}

func nonNilConnections(value []connection) []connection {
	if value == nil {
		return []connection{}
	}
	return value
}

func nonNilRoutes(value []route) []route {
	if value == nil {
		return []route{}
	}
	return value
}
