package pkg

import (
	"context"
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
)

func TestGetPluginInfo(t *testing.T) {
	identityServer := &IdentityServer{}
	resp, err := identityServer.GetPluginInfo(context.Background(), &csi.GetPluginInfoRequest{})
	if err != nil {
		t.Fatalf("GetPluginInfo failed: %v", err)
	}
	if resp.GetName() != DriverName {
		t.Fatalf("Expected name %q, got %q", DriverName, resp.GetName())
	}
	if resp.GetVendorVersion() != DriverVersion {
		t.Fatalf("Expected version %q, got %q", DriverVersion, resp.GetVendorVersion())
	}
}
func TestProbe(t *testing.T) {
	identityServer := &IdentityServer{}
	_, err := identityServer.Probe(context.Background(), &csi.ProbeRequest{})
	if err != nil {
		t.Fatalf("Probe failed: %v", err)
	}
}

func TestGetPluginCapabilities(t *testing.T) {
	identityServer := &IdentityServer{}
	resp, err := identityServer.GetPluginCapabilities(context.Background(), &csi.GetPluginCapabilitiesRequest{})
	if err != nil {
		t.Fatalf("GetPluginCapabilities failed: %v", err)
	}
	if len(resp.GetCapabilities()) != 1 {
		t.Fatalf("Expected 1 capability, got %d", len(resp.GetCapabilities()))
	}
}
