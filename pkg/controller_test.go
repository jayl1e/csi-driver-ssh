package pkg

import (
	"context"
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
)

func newTestDriver() *SshController {
	cfg := ControllerCfg{
		Endpoint:          "unix:///tmp/csi.sock",
		CreateCmd:         `sh ../test/create_volume.sh`,
		DeleteCmd:         "sh ../test/delete_volume.sh",
		CreateSnapshotCmd: "sh ../test/create_snapshot.sh",
		DeleteSnapshotCmd: "sh ../test/delete_snapshot.sh",
		ExpandCmd:         "sh ../test/expand_volume.sh",
	}
	server := NewController(cfg)
	server.executer = &LocalExecuter{}
	return server
}

func TestCreateVolume(t *testing.T) {
	driver := newTestDriver()
	resp, err := driver.CreateVolume(context.Background(), &csi.CreateVolumeRequest{
		Name: "test-volume",
		CapacityRange: &csi.CapacityRange{
			RequiredBytes: 1024 * 1024 * 10, // 10 MB
		},
		Parameters: map[string]string{
			"pk":            "pv",
			"example.com/k": "v",
		},
	})
	if err != nil {
		t.Fatalf("CreateVolume failed: %v", err)
	}
	if resp.Volume == nil {
		t.Fatal("Expected volume in response, got nil")
	}
	if resp.Volume.VolumeId != CSI_VOLUME_ID_PREFIX+"test-volume" {
		t.Errorf("Expected VolumeId %s, got %s", CSI_VOLUME_ID_PREFIX+"test-volume", resp.Volume.VolumeId)
	}
	if resp.Volume.CapacityBytes < 1024*1024*10 {
		t.Errorf("Expected CapacityBytes larger than %d, got %d", 1024*1024*10, resp.Volume.CapacityBytes)
	}
}

func TestDeleteVolume(t *testing.T) {
	driver := newTestDriver()
	_, err := driver.DeleteVolume(context.Background(), &csi.DeleteVolumeRequest{
		VolumeId: CSI_VOLUME_ID_PREFIX + "test-volume",
	})
	if err != nil {
		t.Fatalf("DeleteVolume failed: %v", err)
	}
}

func TestCreateSnapshot(t *testing.T) {
	driver := newTestDriver()
	resp, err := driver.CreateSnapshot(context.Background(), &csi.CreateSnapshotRequest{
		SourceVolumeId: CSI_VOLUME_ID_PREFIX + "test-volume",
		Name:           "snapshot-1234-5678",
	})
	if err != nil {
		t.Fatalf("CreateSnapshot failed: %v", err)
	}
	if resp.Snapshot.SnapshotId != CSI_SNAPSHOT_ID_PREFIX+"snapshot-1234-5678" {
		t.Errorf("Expected SnapshotId %s, got %s", CSI_SNAPSHOT_ID_PREFIX+"snapshot-1234-5678", resp.Snapshot.SnapshotId)
	}
}

func TestDeleteSnapshot(t *testing.T) {
	driver := newTestDriver()
	_, err := driver.DeleteSnapshot(context.Background(), &csi.DeleteSnapshotRequest{
		SnapshotId: CSI_SNAPSHOT_ID_PREFIX + "snapshot-1234-5678",
	})
	if err != nil {
		t.Fatalf("DeleteSnapshot failed: %v", err)
	}
	_, err = driver.DeleteSnapshot(context.Background(), &csi.DeleteSnapshotRequest{
		SnapshotId: "snapshot-1234-5678",
	})
	if err == nil {
		t.Fatalf("DeleteSnapshot should fail for bad id")
	}
}

func TestControllerExpandVolume(t *testing.T) {
	driver := newTestDriver()
	resp, err := driver.ControllerExpandVolume(context.Background(), &csi.ControllerExpandVolumeRequest{
		VolumeId: CSI_VOLUME_ID_PREFIX + "test-volume",
		CapacityRange: &csi.CapacityRange{
			RequiredBytes: 1024 * 1024 * 20, // 20 MB
		},
	})
	if err != nil {
		t.Fatalf("ControllerExpandVolume failed: %v", err)
	}
	if resp.CapacityBytes < 1024*1024*20 {
		t.Errorf("Expected CapacityBytes larger than %d, got %d", 1024*1024*20, resp.CapacityBytes)
	}
}

func TestControllerGetCapabilities(t *testing.T) {
	driver := newTestDriver()
	resp, err := driver.ControllerGetCapabilities(context.Background(), &csi.ControllerGetCapabilitiesRequest{})
	if err != nil {
		t.Fatalf("GetCapabilities failed: %v", err)
	}
	if len(resp.Capabilities) == 0 {
		t.Fatal("Expected capabilities in response, got none")
	}
}
