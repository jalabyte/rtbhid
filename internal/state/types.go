package state

import "time"

type ManualPrefix struct {
	Prefix    string    `json:"prefix"`
	Enabled   bool      `json:"enabled"`
	AddedAt   time.Time `json:"added_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Source struct {
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	URL             string     `json:"url"`
	Enabled         bool       `json:"enabled"`
	IntervalSeconds int        `json:"interval_seconds"`
	TimeoutSeconds  int        `json:"timeout_seconds"`
	LastSyncAt      *time.Time `json:"last_sync_at,omitempty"`
	LastSuccessAt   *time.Time `json:"last_success_at,omitempty"`
	LastError       string     `json:"last_error,omitempty"`
	LastCount       int        `json:"last_count"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type BGPPeer struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Neighbor    string    `json:"neighbor"`
	NeighborASN int       `json:"neighbor_asn"`
	Enabled     bool      `json:"enabled"`
	Description string    `json:"description,omitempty"`
	Password    string    `json:"password,omitempty"`
	Multihop    int       `json:"multihop"`
	Family      string    `json:"family"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type ExcludedPrefix struct {
	Prefix    string    `json:"prefix"`
	Reason    string    `json:"reason,omitempty"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Event struct {
	Time     time.Time `json:"time"`
	Level    string    `json:"level"`
	Message  string    `json:"message"`
	SourceID string    `json:"source_id,omitempty"`
}

type Data struct {
	ManualPrefixes map[string]ManualPrefix   `json:"manual_prefixes"`
	Sources        map[string]Source         `json:"sources"`
	SourcePrefixes map[string]map[string]any `json:"source_prefixes"`
	Peers          map[string]BGPPeer        `json:"peers"`
	Excluded       map[string]ExcludedPrefix `json:"excluded"`
	Events         []Event                   `json:"events"`
}
