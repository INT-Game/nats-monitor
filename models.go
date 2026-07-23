package main

import "time"

type slowConsumerStats struct {
	Clients  int `json:"clients"`
	Routes   int `json:"routes"`
	Gateways int `json:"gateways"`
	Leafs    int `json:"leafs"`
}

type varzResponse struct {
	ServerID          string            `json:"server_id"`
	ServerName        string            `json:"server_name"`
	Version           string            `json:"version"`
	Go                string            `json:"go"`
	Host              string            `json:"host"`
	Port              int               `json:"port"`
	AuthRequired      bool              `json:"auth_required"`
	MaxConnections    int               `json:"max_connections"`
	MaxPayload        int64             `json:"max_payload"`
	Uptime            string            `json:"uptime"`
	CPU               float64           `json:"cpu"`
	Mem               int64             `json:"mem"`
	Cores             int               `json:"cores"`
	GoMaxProcs        int               `json:"gomaxprocs"`
	Connections       int               `json:"connections"`
	TotalConnections  uint64            `json:"total_connections"`
	Subscriptions     uint64            `json:"subscriptions"`
	SlowConsumers     int               `json:"slow_consumers"`
	SlowConsumerStats slowConsumerStats `json:"slow_consumer_stats"`
	InMsgs            uint64            `json:"in_msgs"`
	OutMsgs           uint64            `json:"out_msgs"`
	InBytes           uint64            `json:"in_bytes"`
	OutBytes          uint64            `json:"out_bytes"`
}

type connzResponse struct {
	NumConnections int          `json:"num_connections"`
	Total          int          `json:"total"`
	Offset         int          `json:"offset"`
	Limit          int          `json:"limit"`
	Connections    []connection `json:"connections"`
}

type connection struct {
	CID                 uint64               `json:"cid"`
	Kind                string               `json:"kind,omitempty"`
	Type                string               `json:"type,omitempty"`
	Name                string               `json:"name,omitempty"`
	IP                  string               `json:"ip"`
	Port                int                  `json:"port"`
	Account             string               `json:"account,omitempty"`
	AuthorizedUser      string               `json:"authorized_user,omitempty"`
	Lang                string               `json:"lang,omitempty"`
	Version             string               `json:"version,omitempty"`
	Start               time.Time            `json:"start"`
	LastActivity        time.Time            `json:"last_activity"`
	Stop                time.Time            `json:"stop,omitempty"`
	Reason              string               `json:"reason,omitempty"`
	RTT                 string               `json:"rtt,omitempty"`
	Uptime              string               `json:"uptime"`
	Idle                string               `json:"idle"`
	PendingBytes        int64                `json:"pending_bytes"`
	InMsgs              int64                `json:"in_msgs"`
	OutMsgs             int64                `json:"out_msgs"`
	InBytes             int64                `json:"in_bytes"`
	OutBytes            int64                `json:"out_bytes"`
	Subscriptions       uint32               `json:"subscriptions"`
	SubscriptionDetails []subscriptionDetail `json:"subscriptions_list_detail,omitempty"`
}

type subscriptionDetail struct {
	Account    string `json:"account,omitempty"`
	AccountTag string `json:"account_tag,omitempty"`
	Subject    string `json:"subject"`
	Queue      string `json:"qgroup,omitempty"`
	SID        string `json:"sid"`
	Messages   int64  `json:"msgs"`
	Max        int64  `json:"max,omitempty"`
	CID        uint64 `json:"cid"`
}

type subszResponse struct {
	NumSubscriptions uint64               `json:"num_subscriptions"`
	NumCache         uint64               `json:"num_cache"`
	NumInserts       uint64               `json:"num_inserts"`
	NumRemoves       uint64               `json:"num_removes"`
	NumMatches       uint64               `json:"num_matches"`
	CacheHitRate     float64              `json:"cache_hit_rate"`
	MaxFanout        uint64               `json:"max_fanout"`
	AvgFanout        float64              `json:"avg_fanout"`
	Total            int                  `json:"total"`
	Offset           int                  `json:"offset"`
	Limit            int                  `json:"limit"`
	List             []subscriptionDetail `json:"subscriptions_list"`
}

type routezResponse struct {
	NumRoutes int     `json:"num_routes"`
	Routes    []route `json:"routes"`
}

type route struct {
	RID                 uint64               `json:"rid"`
	RemoteID            string               `json:"remote_id"`
	RemoteName          string               `json:"remote_name"`
	DidSolicit          bool                 `json:"did_solicit"`
	IsConfigured        bool                 `json:"is_configured"`
	IP                  string               `json:"ip"`
	Port                int                  `json:"port"`
	Start               time.Time            `json:"start"`
	LastActivity        time.Time            `json:"last_activity"`
	RTT                 string               `json:"rtt,omitempty"`
	Uptime              string               `json:"uptime"`
	Idle                string               `json:"idle"`
	PendingSize         int64                `json:"pending_size"`
	InMsgs              int64                `json:"in_msgs"`
	OutMsgs             int64                `json:"out_msgs"`
	InBytes             int64                `json:"in_bytes"`
	OutBytes            int64                `json:"out_bytes"`
	Subscriptions       uint32               `json:"subscriptions"`
	SubscriptionDetails []subscriptionDetail `json:"subscriptions_list_detail,omitempty"`
}

type gatewayzResponse struct {
	Name             string                     `json:"name,omitempty"`
	Host             string                     `json:"host,omitempty"`
	Port             int                        `json:"port,omitempty"`
	OutboundGateways map[string]remoteGateway   `json:"outbound_gateways"`
	InboundGateways  map[string][]remoteGateway `json:"inbound_gateways"`
}

type remoteGateway struct {
	Configured bool        `json:"configured"`
	Connection *connection `json:"connection,omitempty"`
}

type leafzResponse struct {
	NumLeafs int        `json:"leafnodes"`
	Leafs    []leafInfo `json:"leafs"`
}

type leafInfo struct {
	ID            uint64   `json:"id"`
	Name          string   `json:"name"`
	IsSpoke       bool     `json:"is_spoke"`
	IsIsolated    bool     `json:"is_isolated,omitempty"`
	Account       string   `json:"account"`
	IP            string   `json:"ip"`
	Port          int      `json:"port"`
	RTT           string   `json:"rtt,omitempty"`
	InMsgs        int64    `json:"in_msgs"`
	OutMsgs       int64    `json:"out_msgs"`
	InBytes       int64    `json:"in_bytes"`
	OutBytes      int64    `json:"out_bytes"`
	Subscriptions uint32   `json:"subscriptions"`
	Subjects      []string `json:"subscriptions_list,omitempty"`
	Compression   string   `json:"compression,omitempty"`
}

type accountzResponse struct {
	SystemAccount string   `json:"system_account"`
	Accounts      []string `json:"accounts"`
}

type dataStats struct {
	Messages int64 `json:"msgs"`
	Bytes    int64 `json:"bytes"`
}

type accountStat struct {
	Account          string    `json:"acc"`
	Name             string    `json:"name,omitempty"`
	Connections      int       `json:"conns"`
	LeafNodes        int       `json:"leafnodes"`
	TotalConnections int       `json:"total_conns"`
	Subscriptions    int       `json:"num_subscriptions"`
	Sent             dataStats `json:"sent"`
	Received         dataStats `json:"received"`
	SlowConsumers    int       `json:"slow_consumers"`
}

type accountStatzResponse struct {
	Accounts []accountStat `json:"account_statz"`
}

type jetStreamAPIStats struct {
	Total  int64 `json:"total"`
	Errors int64 `json:"errors"`
}

type jetStreamResponse struct {
	Disabled        bool              `json:"disabled"`
	Memory          int64             `json:"memory"`
	Storage         int64             `json:"storage"`
	ReservedMemory  int64             `json:"reserved_memory"`
	ReservedStorage int64             `json:"reserved_storage"`
	Accounts        int               `json:"accounts"`
	HAAssets        int               `json:"ha_assets"`
	Streams         int               `json:"streams"`
	Consumers       int               `json:"consumers"`
	Messages        int64             `json:"messages"`
	Bytes           int64             `json:"bytes"`
	API             jetStreamAPIStats `json:"api"`
}

type rates struct {
	MessagesPerSecond float64 `json:"messages_per_second"`
	BytesPerSecond    float64 `json:"bytes_per_second"`
}

type historyPoint struct {
	At          time.Time `json:"at"`
	MessagesPS  float64   `json:"messages_ps"`
	BytesPS     float64   `json:"bytes_ps"`
	Connections int       `json:"connections"`
	CPU         float64   `json:"cpu"`
	Memory      int64     `json:"memory"`
}

type snapshot struct {
	Viewer                   string            `json:"viewer,omitempty"`
	Target                   string            `json:"target"`
	Targets                  []monitorTarget   `json:"targets,omitempty"`
	Healthy                  bool              `json:"healthy"`
	Error                    string            `json:"error,omitempty"`
	Warnings                 []string          `json:"warnings"`
	LastAttemptAt            time.Time         `json:"last_attempt_at"`
	LastSuccessAt            time.Time         `json:"last_success_at"`
	Server                   varzResponse      `json:"server"`
	Rates                    rates             `json:"rates"`
	Connections              []connection      `json:"connections"`
	RecentSlowConsumers      []connection      `json:"recent_slow_consumers"`
	ClosedConnectionsScanned int               `json:"closed_connections_scanned"`
	ClosedConnectionsTotal   int               `json:"closed_connections_total"`
	Subscriptions            subszResponse     `json:"subscriptions"`
	Routes                   []route           `json:"routes"`
	Gateways                 gatewayzResponse  `json:"gateways"`
	LeafNodes                leafzResponse     `json:"leaf_nodes"`
	Accounts                 accountzResponse  `json:"accounts"`
	AccountStats             []accountStat     `json:"account_stats"`
	JetStream                jetStreamResponse `json:"jetstream"`
	History                  []historyPoint    `json:"history"`
}
