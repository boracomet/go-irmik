// Package grpcx provides thin gRPC server/client bootstrap helpers.
//
// Opt-in: google.golang.org/grpc is only linked when this package is imported.
// Do not import grpcx from the root irmik package.
package grpcx

import (
	"context"
	"fmt"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

// ServerOptions configures ListenAndServe.
type ServerOptions struct {
	// Addr is host:port (default ":50051"). Use "127.0.0.1:0" for tests.
	Addr string
	// EnableReflection registers gRPC reflection (useful in dev).
	EnableReflection bool
	// EnableHealth registers the standard health service (SERVING).
	EnableHealth bool
	// ServerOptions are passed to grpc.NewServer.
	ServerOptions []grpc.ServerOption
	// Register is called with the server to register service implementations.
	Register func(*grpc.Server)
}

// Server wraps grpc.Server and its listener.
type Server struct {
	GRPC *grpc.Server
	lis  net.Listener
	addr string
}

// NewServer builds a gRPC server (does not listen yet).
func NewServer(opts ServerOptions) *Server {
	gs := grpc.NewServer(opts.ServerOptions...)
	if opts.EnableHealth {
		hs := health.NewServer()
		hs.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
		healthpb.RegisterHealthServer(gs, hs)
	}
	if opts.EnableReflection {
		reflection.Register(gs)
	}
	if opts.Register != nil {
		opts.Register(gs)
	}
	addr := opts.Addr
	if addr == "" {
		addr = ":50051"
	}
	return &Server{GRPC: gs, addr: addr}
}

// ListenAndServe listens and serves until ctx is cancelled, then GracefulStop.
func (s *Server) ListenAndServe(ctx context.Context) error {
	lis, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("grpcx: listen: %w", err)
	}
	s.lis = lis
	errCh := make(chan error, 1)
	go func() {
		errCh <- s.GRPC.Serve(lis)
	}()
	select {
	case <-ctx.Done():
		stopped := make(chan struct{})
		go func() {
			s.GRPC.GracefulStop()
			close(stopped)
		}()
		select {
		case <-stopped:
		case <-time.After(10 * time.Second):
			s.GRPC.Stop()
		}
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}

// Addr returns the bound address once listening, else the configured addr.
func (s *Server) Addr() string {
	if s.lis != nil {
		return s.lis.Addr().String()
	}
	return s.addr
}

// DialOptions configures Dial.
type DialOptions struct {
	// Insecure uses insecure credentials (default true).
	Insecure *bool
	// DialOptions are passed to grpc.NewClient.
	DialOptions []grpc.DialOption
}

// Dial creates a client connection to target (e.g. "localhost:50051").
func Dial(_ context.Context, target string, opts DialOptions) (*grpc.ClientConn, error) {
	dopts := append([]grpc.DialOption{}, opts.DialOptions...)
	insecureDefault := opts.Insecure == nil || *opts.Insecure
	if insecureDefault {
		dopts = append(dopts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	return grpc.NewClient(target, dopts...)
}
