package grpcx

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func TestServerHealth(t *testing.T) {
	s := NewServer(ServerOptions{
		Addr:         "127.0.0.1:0",
		EnableHealth: true,
	})
	// Bind first to get port via manual listen path: use ListenAndServe with short ctx.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Replace addr with ephemeral by listening ourselves through ListenAndServe race:
	// Start in background; Addr may still be :0 until listen — use fixed free approach.
	lisCtx, lisCancel := context.WithCancel(context.Background())
	defer lisCancel()
	errCh := make(chan error, 1)
	go func() { errCh <- s.ListenAndServe(lisCtx) }()

	// Wait briefly for listen
	deadline := time.Now().Add(2 * time.Second)
	var addr string
	for time.Now().Before(deadline) {
		addr = s.Addr()
		if addr != "" && addr != "127.0.0.1:0" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if addr == "" || addr == "127.0.0.1:0" {
		t.Fatal("server did not bind")
	}

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	cli := healthpb.NewHealthClient(conn)
	resp, err := cli.Check(ctx, &healthpb.HealthCheckRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != healthpb.HealthCheckResponse_SERVING {
		t.Fatalf("status=%v", resp.Status)
	}
	lisCancel()
	select {
	case <-errCh:
	case <-time.After(3 * time.Second):
		t.Fatal("shutdown timeout")
	}
}
