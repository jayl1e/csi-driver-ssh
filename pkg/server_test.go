package pkg

import (
	"testing"
)

func TestNewGrpcServer(t *testing.T) {
	s, e := NewGrpcServer("unix:///tmp/csi.sock")
	if e != nil {
		t.Fatalf("failed to create gRPC server: %v", e)
	}
	if s == nil {
		t.Fatal("gRPC server is nil")
	}
}
