package pkg

import (
	"context"
	"os"
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
	mount "k8s.io/mount-utils"
)

func newTestNode(mounter mount.Interface) *SshNodeServer {
	cfg := NodeCfg{
		Endpoint:        "unix:///tmp/csi.sock",
		MountPermission: 0755,
		NodeID:          "test-node",
	}
	server := NewNodeServer(cfg)
	server.mounter = mounter
	return server
}

func TestNodePublishVolume(t *testing.T) {
	mockMounter := mount.NewFakeMounter([]mount.MountPoint{})
	driver := newTestNode(mockMounter)
	_, err := driver.NodePublishVolume(context.Background(), &csi.NodePublishVolumeRequest{
		VolumeId:         "test-volume",
		TargetPath:       "/tmp",
		VolumeCapability: &csi.VolumeCapability{},
		VolumeContext:    map[string]string{"nfs_server": "test-server", "nfs_path": "/test/path"},
	})
	if err != nil {
		t.Fatalf("NodePublishVolume failed: %v", err)
	}
	logs := mockMounter.GetLog()
	if len(logs) != 1 {
		t.Fatalf("Expected 1 log entry, got %d: %+v", len(logs), logs)
	}
}

func TestNodeUnpublishVolume(t *testing.T) {
	temp_mount_dir, err := os.MkdirTemp(os.TempDir(), "test-")
	defer os.RemoveAll(temp_mount_dir)
	mockMounter := mount.NewFakeMounter([]mount.MountPoint{
		{Type: "nfs", Path: temp_mount_dir},
	})
	mockMounter.WithSkipMountPointCheck()
	driver := newTestNode(mockMounter)
	_, err = driver.NodeUnpublishVolume(context.Background(), &csi.NodeUnpublishVolumeRequest{
		VolumeId:   "test-volume",
		TargetPath: temp_mount_dir,
	})
	if err != nil {
		t.Fatalf("NodeUnpublishVolume failed: %v", err)
	}
	logs := mockMounter.GetLog()
	if len(logs) != 1 {
		t.Fatalf("Expected 1 log entry, got %d: %+v", len(logs), logs)
	}
}

func TestNodeGetCapabilities(t *testing.T) {
	driver := newTestNode(nil)
	resp, err := driver.NodeGetCapabilities(context.Background(), &csi.NodeGetCapabilitiesRequest{})
	if err != nil {
		t.Fatalf("NodeGetCapabilities failed: %v", err)
	}
	if len(resp.Capabilities) != 2 {
		t.Fatalf("Expected 2 capabilities, got %d: %+v", len(resp.Capabilities), resp.Capabilities)
	}
}
