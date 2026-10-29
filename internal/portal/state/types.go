package state

import "time"

type Agent struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Hostname    string     `json:"hostname"`
	BirdVersion string     `json:"bird_version"`
	ListenAddr  string     `json:"listen_addr"`
	Status      string     `json:"status"`
	LastSeen    *time.Time `json:"last_seen"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type Prefix struct {
	ID        int64     `json:"id"`
	CIDR      string    `json:"cidr"`
	Enabled   bool      `json:"enabled"`
	Note      string    `json:"note"`
	CreatedAt time.Time `json:"created_at"`
}

type Source struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	URL       string     `json:"url"`
	Interval  int        `json:"interval"`
	Enabled   bool       `json:"enabled"`
	LastSync  *time.Time `json:"last_sync"`
	LastCount int        `json:"last_count"`
	CreatedAt time.Time  `json:"created_at"`
}

type Exclude struct {
	ID        int64     `json:"id"`
	CIDR      string    `json:"cidr"`
	Note      string    `json:"note"`
	CreatedAt time.Time `json:"created_at"`
}

type Peer struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	NeighborIP       string    `json:"neighbor_ip"`
	RemoteAS         int       `json:"remote_as"`
	LocalAS          int       `json:"local_as"`
	Multihop         int       `json:"multihop"`
	Enabled          bool      `json:"enabled"`
	StdCommunities   []string  `json:"std_communities"`
	LargeCommunities []string  `json:"large_communities"`
	CreatedAt        time.Time `json:"created_at"`
}

type Event struct {
	ID        int64     `json:"id"`
	Type      string    `json:"type"`
	Message   string    `json:"message"`
	AgentID   string    `json:"agent_id"`
	CreatedAt time.Time `json:"created_at"`
}

// WhitelistASN is an ASN allowed to auto-peer with RTBHID agents over BGP.
type WhitelistASN struct {
	ID        string    `json:"id"`
	ASN       uint32    `json:"asn"`
	Name      string    `json:"name"`
	Country   string    `json:"country"`
	Source    string    `json:"source"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
}

// BGPSession is a live BGP session reported by an agent.
type BGPSession struct {
	AgentID    string    `json:"agent_id"`
	AgentName  string    `json:"agent_name,omitempty"`
	Peer       string    `json:"peer"`
	Name       string    `json:"name"`
	RemoteAS   uint32    `json:"remote_as"`
	LocalAS    uint32    `json:"local_as"`
	State      string    `json:"state"`
	Passive    bool      `json:"passive"`
	UptimeSec  int64     `json:"uptime_sec"`
	PrefixesIn int       `json:"prefixes_in"`
	LastError  string    `json:"last_error"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// LookupMatch represents a single lookup hit.
type LookupMatch struct {
	Type       string    `json:"type"`
	SourceID   string    `json:"source_id"`
	SourceName string    `json:"source_name"`
	CIDR       string    `json:"cidr"`
	Note       string    `json:"note"`
	CreatedAt  time.Time `json:"created_at"`
	ASN        string    `json:"asn,omitempty"`
	ASName     string    `json:"as_name,omitempty"`
	ASDomain   string    `json:"as_domain,omitempty"`
}

type LookupResult struct {
	Query   string        `json:"query"`
	Matches []LookupMatch `json:"matches"`
}

type LookupResponse struct {
	Results []LookupResult `json:"results"`
}
