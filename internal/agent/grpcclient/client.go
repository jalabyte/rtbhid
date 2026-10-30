// Package grpcclient provides a gRPC client for agents to connect to the portal.
package grpcclient

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	pb "rtbhid/gen/rtbhid"
	"rtbhid/internal/agent/bird"
	"rtbhid/internal/agent/engine"
)

// Config holds the agent gRPC client configuration.
type Config struct {
	PortalURL string
	// PortalTLS dials the portal over TLS (system roots), e.g. through Caddy.
	PortalTLS bool
	// Token is the enrollment token, sent as a bearer token on every call.
	// Agents on the portal host itself may leave it empty.
	Token        string
	AgentID      string
	AgentName    string
	Hostname     string
	BirdVersion  string
	AgentVersion string

	BirdOutputPath    string
	BirdPeersOutput   string
	BirdReloadEnabled bool
	BirdCommand       []string
	LocalASN          int
	ExportStdComms    string
	ExportLargeComms  string

	// BGPEngine selects the routing implementation: "bird" (default) or
	// "custom" (embedded BGP server).
	BGPEngine      string
	BGPListenAddr  string
	BGPRouterID    string
	BGPNexthopV4   string
	BGPNexthopV6   string
	BGPHoldTimeSec int
	BGPEnableIPv6  bool

	// GRPCMaxRecvMsgSize is the maximum message size in bytes the client
	// will accept. If 0, uses gRPC default (4MB).
	GRPCMaxRecvMsgSize int
}

// Client connects to the portal and manages routing state locally.
type Client struct {
	cfg        Config
	conn       *grpc.ClientConn
	portal     pb.PortalClient
	engine     engine.Engine
	mu         sync.Mutex
	lastConfig *pb.ConfigUpdate
}

// New creates a new gRPC agent client and starts the configured BGP engine.
func New(cfg Config) (*Client, error) {
	var eng engine.Engine
	switch strings.ToLower(strings.TrimSpace(cfg.BGPEngine)) {
	case "", "bird":
		exportPolicy, _ := bird.ParseExportPolicy(cfg.ExportStdComms, cfg.ExportLargeComms)
		birdMgr := bird.NewManager(
			cfg.BirdOutputPath,
			cfg.BirdPeersOutput,
			cfg.BirdReloadEnabled,
			cfg.BirdCommand,
			exportPolicy,
		)
		eng = engine.NewBird(birdMgr, cfg.LocalASN)
	case "custom", "embedded":
		custom, err := engine.NewCustom(engine.CustomConfig{
			LocalASN:         cfg.LocalASN,
			ListenAddr:       cfg.BGPListenAddr,
			RouterID:         cfg.BGPRouterID,
			NexthopV4:        cfg.BGPNexthopV4,
			NexthopV6:        cfg.BGPNexthopV6,
			HoldTimeSec:      cfg.BGPHoldTimeSec,
			EnableIPv6:       cfg.BGPEnableIPv6,
			ExportStdComms:   cfg.ExportStdComms,
			ExportLargeComms: cfg.ExportLargeComms,
			Logger:           log.Default(),
		})
		if err != nil {
			return nil, fmt.Errorf("start custom BGP engine: %w", err)
		}
		eng = custom
	default:
		return nil, fmt.Errorf("unknown RTBH_BGP_ENGINE %q (expected bird or custom)", cfg.BGPEngine)
	}
	return &Client{cfg: cfg, engine: eng}, nil
}

// Run is the main agent loop: register, heartbeat, config stream.
func (c *Client) Run(ctx context.Context) error {
	defer c.engine.Close()

	agentID, err := c.loadOrGenerateID()
	if err != nil {
		return fmt.Errorf("load agent ID: %w", err)
	}
	c.cfg.AgentID = agentID

	maxRecvSize := c.cfg.GRPCMaxRecvMsgSize
	if maxRecvSize <= 0 {
		maxRecvSize = 4 * 1024 * 1024 // gRPC default
	}
	transport := insecure.NewCredentials()
	if c.cfg.PortalTLS {
		transport = credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12})
	}
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(transport),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(maxRecvSize)),
	}
	if c.cfg.Token != "" {
		opts = append(opts, grpc.WithPerRPCCredentials(bearerToken{c.cfg.Token, c.cfg.PortalTLS}))
	}
	conn, err := grpc.NewClient(c.cfg.PortalURL, opts...)
	if err != nil {
		return fmt.Errorf("connect to portal: %w", err)
	}
	c.conn = conn
	c.portal = pb.NewPortalClient(conn)

	regResp, err := c.portal.Register(ctx, &pb.RegisterRequest{
		AgentId:     c.cfg.AgentID,
		Name:        c.cfg.AgentName,
		Hostname:    c.cfg.Hostname,
		BirdVersion: c.cfg.BirdVersion,
		Version:     c.cfg.AgentVersion,
	})
	if err != nil {
		return fmt.Errorf("register: %w", err)
	}
	if !regResp.Success {
		return fmt.Errorf("registration failed: %s", regResp.Message)
	}
	log.Printf("registered with portal, heartbeat interval: %ds", regResp.HeartbeatInterval)

	heartbeatInterval := time.Duration(regResp.HeartbeatInterval) * time.Second
	go c.heartbeatLoop(ctx, heartbeatInterval)

	return c.configStreamLoop(ctx)
}

func (c *Client) heartbeatLoop(ctx context.Context, interval time.Duration) {
	// Report once immediately so the portal has data without waiting a full
	// interval.
	c.sendHeartbeat(ctx)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.sendHeartbeat(ctx)
		}
	}
}

func (c *Client) sendHeartbeat(ctx context.Context) {
	c.mu.Lock()
	prefixCount := 0
	peerCount := 0
	if c.lastConfig != nil {
		prefixCount = len(c.lastConfig.Prefixes)
		peerCount = len(c.lastConfig.Peers)
	}
	c.mu.Unlock()

	engineName := c.cfg.BGPEngine
	var sessions []*pb.BgpSession
	if reporter, ok := c.engine.(engine.Reporter); ok {
		name, list := reporter.BGPStatus()
		if name != "" {
			engineName = name
		}
		sessions = make([]*pb.BgpSession, 0, len(list))
		for _, s := range list {
			sessions = append(sessions, &pb.BgpSession{
				Peer:        s.Peer,
				Name:        s.Name,
				RemoteAs:    s.RemoteAS,
				LocalAs:     s.LocalAS,
				State:       s.State,
				Passive:     s.Passive,
				UptimeSec:   s.UptimeSec,
				PrefixesIn:  int32(s.PrefixesIn),
				PrefixesOut: int32(s.PrefixesOut),
				LastError:   s.LastError,
			})
		}
	}

	_, err := c.portal.Heartbeat(ctx, &pb.HeartbeatRequest{
		AgentId:     c.cfg.AgentID,
		PrefixCount: int32(prefixCount),
		PeerCount:   int32(peerCount),
		BirdStatus:  "running",
		BgpSessions: sessions,
		BgpEngine:   engineName,
	})
	if err != nil {
		log.Printf("heartbeat error: %v", err)
	}
}

func (c *Client) configStreamLoop(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		stream, err := c.portal.StreamConfig(ctx, &pb.ConfigRequest{AgentId: c.cfg.AgentID})
		if err != nil {
			log.Printf("stream config error: %v, retrying in 5s", err)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(5 * time.Second):
				continue
			}
		}

		for {
			update, err := stream.Recv()
			if err == io.EOF {
				log.Println("config stream ended, reconnecting")
				break
			}
			if err != nil {
				log.Printf("config stream error: %v, reconnecting in 5s", err)
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(5 * time.Second):
				}
				break
			}
			c.applyConfig(update)
		}
	}
}

func (c *Client) applyConfig(update *pb.ConfigUpdate) {
	c.mu.Lock()
	c.lastConfig = update
	c.mu.Unlock()

	log.Printf("applying config version %d: %d prefixes, %d peers, %d whitelisted ASNs (auto_accept=%v)",
		update.Version, len(update.Prefixes), len(update.Peers), len(update.AllowedAsns), update.AutoAccept)

	peers := make([]bird.Peer, 0, len(update.Peers))
	for _, p := range update.Peers {
		peers = append(peers, bird.Peer{
			ID:          p.Id,
			Name:        p.Name,
			Neighbor:    p.NeighborIp,
			NeighborASN: int(p.RemoteAs),
			Enabled:     true,
			Multihop:    int(p.Multihop),
		})
	}

	result, err := c.engine.Apply(update.Prefixes, peers, update.AllowedAsns, update.AutoAccept)
	if err != nil {
		log.Printf("apply error: %v", err)
		return
	}
	log.Printf("config applied: %d routes, %d peers, reload=%v",
		result.RouteCount, result.PeerCount, result.ReloadRan)
}

func (c *Client) loadOrGenerateID() (string, error) {
	idPath := "./data/agent_id"
	if data, err := os.ReadFile(idPath); err == nil {
		id := string(data)
		if len(id) > 0 {
			return id, nil
		}
	}

	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	id := fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])

	if err := os.MkdirAll("./data", 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(idPath, []byte(id), 0o644); err != nil {
		return "", err
	}
	log.Printf("generated new agent ID: %s", id)
	return id, nil
}

type bearerToken struct {
	token  string
	secure bool
}

func (b bearerToken) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return map[string]string{"authorization": "Bearer " + b.token}, nil
}

// RequireTransportSecurity refuses to send the token in clear text unless TLS
// is off on purpose (a portal reached over a private network).
func (b bearerToken) RequireTransportSecurity() bool { return b.secure }
