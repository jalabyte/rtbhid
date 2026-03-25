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

type LookupMatch struct {
	Type       string    `json:"type"`
	SourceID   string    `json:"source_id"`
	SourceName string    `json:"source_name"`
	CIDR       string    `json:"cidr"`
	Note       string    `json:"note"`
	CreatedAt  time.Time `json:"created_at"`
}

type LookupResult struct {
	Query   string        `json:"query"`
	Matches []LookupMatch `json:"matches"`
}

type LookupResponse struct {
	Results []LookupResult `json:"results"`
}
