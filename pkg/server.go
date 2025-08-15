package pkg

import (
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"google.golang.org/grpc"
)

type GrpcServer struct {
	Endpoint string
	server   *grpc.Server
	listener net.Listener
}

func NewGrpcServer(endpoint string) (*GrpcServer, error) {
	scheme, addr, err := parseEndpoint(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to parse endpoint: %w", err)
	}

	if scheme == "unix" {
		if err := os.Remove(addr); err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to remove existing socket file: %w", err)
		}
	}

	listener, err := net.Listen(scheme, addr)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on %s://%s: %w", scheme, addr, err)
	}
	d := &GrpcServer{}
	d.server = grpc.NewServer()
	d.listener = listener
	return d, nil
}

func (d *GrpcServer) Run() error {
	d.server.Serve(d.listener)
	go d.handleShutdown()
	return nil
}

func (d *GrpcServer) handleShutdown() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	slog.Warn("Received shutdown signal, gracefully stopping...")
	d.server.GracefulStop()
}

func parseEndpoint(endpoint string) (string, string, error) {
	parts := strings.SplitN(endpoint, "://", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid endpoint format: %s", endpoint)
	}
	return parts[0], parts[1], nil
}
