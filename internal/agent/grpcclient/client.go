// Package grpcclient provides a gRPC client for agents to connect to the portal.
package grpcclient

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "rtbhid/gen/rtbhid"
	"rtbhid/internal/agent/bird"
)

// Config holds the agent gRPC client configuration.
type Config struct {
	PortalURL    string
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
}

// Client connects to the portal and manages Bird config locally.
type Client struct {
	cfg        Config
	conn       *grpc.ClientConn
	portal     pb.PortalClient
	birdMgr    *bird.Manager
	mu         sync.Mutex
	lastConfig *pb.ConfigUpdate
}

// New creates a new gRPC agent client.
func New(cfg Config) *Client {
	exportPolicy, _ := bird.ParseExportPolicy(cfg.ExportStdComms, cfg.ExportLargeComms)
	birdMgr := bird.NewManager(
		cfg.BirdOutputPath,
		cfg.BirdPeersOutput,
		cfg.BirdReloadEnabled,
		cfg.BirdCommand,
		exportPolicy,
	)
	return &Client{cfg: cfg, birdMgr: birdMgr}
}

// Run is the main agent loop: register, heartbeat, config stream.
func (c *Client) Run(ctx context.Context) error {
	agentID, err := c.loadOrGenerateID()
	if err != nil {
		return fmt.Errorf("load agent ID: %w", err)
	}
	c.cfg.AgentID = agentID

	conn, err := grpc.NewClient(c.cfg.PortalURL,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return fmt.Errorf("connect to portal: %w", err)
	}
	c.conn = conn
	c.portal = pb.NewPortalClient(conn)

	regResp, err := c.portal.Register(ctx, &pb.RegisterRequest{
		AgentId:     c.cfg.AgentID,
		Name:        c.cfg.AgentName,
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
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.mu.Lock()
			prefixCount := 0
			peerCount := 0
			if c.lastConfig != nil {
				prefixCount = len(c.lastConfig.Prefixes)
				peerCount = len(c.lastConfig.Peers)
			}
			c.mu.Unlock()

			_, err := c.portal.Heartbeat(ctx, &pb.HeartbeatRequest{
				AgentId:     c.cfg.AgentID,
				PrefixCount: int32(prefixCount),
				PeerCount:   int32(peerCount),
				BirdStatus:  "running",
			})
			if err != nil {
				log.Printf("heartbeat error: %v", err)
			}
		}
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

	log.Printf("applying config version %d: %d prefixes, %d peers",
		update.Version, len(update.Prefixes), len(update.Peers))

	peers := make([]bird.Peer, 0, len(update.Peers))
	for _, p := range update.Peers {
		peers = append(peers, bird.Peer{
			ID:          p.Name,
			Name:        p.Name,
			Neighbor:    p.NeighborIp,
			NeighborASN: int(p.RemoteAs),
			Enabled:     true,
			Multihop:    int(p.Multihop),
		})
	}

	result, err := c.birdMgr.Apply(update.Prefixes, peers, c.cfg.LocalASN)
	if err != nil {
		log.Printf("bird apply error: %v", err)
		return
	}
	log.Printf("bird config applied: %d routes, %d peers, reload=%v",
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
