// Package grpcapi provides the portal gRPC server for agent communication.
package grpcapi

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/netip"
	"strings"

	pb "rtbhid/gen/rtbhid"
	"rtbhid/internal/portal"
	"rtbhid/internal/portal/state"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// Server implements the gRPC Portal service.
type Server struct {
	pb.UnimplementedPortalServer

	svc            *portal.Service
	addr           string
	maxSendMsgSize int
}

// NewServer creates a new gRPC portal server.
func NewServer(svc *portal.Service, addr string, maxSendMsgSize int) *Server {
	return &Server{
		svc:            svc,
		addr:           addr,
		maxSendMsgSize: maxSendMsgSize,
	}
}

// ListenAndServe creates a gRPC server, registers the portal service,
// and listens on the configured address. It blocks until the server exits.
func (s *Server) ListenAndServe() error {
	lis, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("grpcapi: listen %s: %w", s.addr, err)
	}

	var opts []grpc.ServerOption
	if s.maxSendMsgSize > 0 {
		opts = append(opts, grpc.MaxSendMsgSize(s.maxSendMsgSize))
	}
	gs := grpc.NewServer(opts...)
	pb.RegisterPortalServer(gs, s)

	log.Printf("gRPC server listening on %s (max send msg: %d bytes)", s.addr, s.maxSendMsgSize)
	return gs.Serve(lis)
}

// Register handles agent registration. It calls the portal service to
// register the agent and returns a success response with a heartbeat
// interval of 30 seconds.
func (s *Server) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	if err := s.authorize(ctx, req.GetAgentId()); err != nil {
		return nil, err
	}
	err := s.svc.HandleRegister(
		req.GetAgentId(),
		req.GetName(),
		req.GetHostname(),
		req.GetBirdVersion(),
		req.GetVersion(),
	)
	if err != nil {
		return &pb.RegisterResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	log.Printf("agent %s (%s) registered", req.GetAgentId(), req.GetName())
	return &pb.RegisterResponse{
		Success:           true,
		Message:           "registered",
		HeartbeatInterval: 15,
	}, nil
}

// Heartbeat handles agent health reports. It always returns success.
func (s *Server) Heartbeat(ctx context.Context, req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
	if err := s.authorize(ctx, req.GetAgentId()); err != nil {
		return nil, err
	}
	sessions := make([]state.BGPSession, 0, len(req.GetBgpSessions()))
	for _, sess := range req.GetBgpSessions() {
		sessions = append(sessions, state.BGPSession{
			Peer:        sess.GetPeer(),
			Name:        sess.GetName(),
			RemoteAS:    sess.GetRemoteAs(),
			LocalAS:     sess.GetLocalAs(),
			State:       sess.GetState(),
			Passive:     sess.GetPassive(),
			UptimeSec:   sess.GetUptimeSec(),
			PrefixesIn:  int(sess.GetPrefixesIn()),
			PrefixesOut: int(sess.GetPrefixesOut()),
			LastError:   sess.GetLastError(),
		})
	}
	s.svc.HandleHeartbeat(
		req.GetAgentId(),
		int(req.GetPrefixCount()),
		int(req.GetPeerCount()),
		req.GetBirdStatus(),
		req.GetBgpEngine(),
		sessions,
	)

	return &pb.HeartbeatResponse{
		Success: true,
	}, nil
}

// StreamConfig implements server-side streaming to push config updates
// to the agent. It registers a stream channel, loops sending updates
// from the channel, and unregisters on exit.
func (s *Server) StreamConfig(req *pb.ConfigRequest, stream pb.Portal_StreamConfigServer) error {
	agentID := req.GetAgentId()
	if err := s.authorize(stream.Context(), agentID); err != nil {
		return err
	}

	ch := s.svc.RegisterStream(agentID)
	defer s.svc.UnregisterStream(agentID)

	log.Printf("agent %s config stream started", agentID)

	for update := range ch {
		if err := stream.Send(update); err != nil {
			log.Printf("agent %s config stream send error: %v", agentID, err)
			return err
		}
	}

	log.Printf("agent %s config stream closed", agentID)
	return nil
}

// authorize admits an agent by its bearer token. An agent on the portal host
// itself (loopback peer, not forwarded by a proxy) may connect without one,
// which keeps single-box installs working. Caddy always adds
// X-Forwarded-For, so proxied remote agents never match that exemption.
func (s *Server) authorize(ctx context.Context, agentID string) error {
	if agentID == "" {
		return status.Error(codes.InvalidArgument, "agent_id is required")
	}
	md, _ := metadata.FromIncomingContext(ctx)
	token := ""
	if v := md.Get("authorization"); len(v) > 0 {
		token = strings.TrimSpace(strings.TrimPrefix(v[0], "Bearer "))
	}
	if token == "" && len(md.Get("x-forwarded-for")) == 0 && isLoopbackPeer(ctx) {
		return nil
	}
	if err := s.svc.AuthorizeAgent(token, agentID); err != nil {
		if errors.Is(err, portal.ErrAgentUnauthorized) {
			return status.Error(codes.Unauthenticated, err.Error())
		}
		return status.Error(codes.Internal, err.Error())
	}
	return nil
}

func isLoopbackPeer(ctx context.Context) bool {
	p, ok := peer.FromContext(ctx)
	if !ok || p.Addr == nil {
		return false
	}
	ap, err := netip.ParseAddrPort(p.Addr.String())
	return err == nil && ap.Addr().Unmap().IsLoopback()
}
