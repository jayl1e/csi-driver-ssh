package pkg

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
)

const (
	DriverName    = "csi-ssh.jayjaylee.com"
	DriverVersion = "0.0.1"
)

const (
	CSI_SHELL_OUTPUT_PREFIX = "csi-shell-output:"
	CSI_REP_VOLUME_ID       = "volume_id"
	CSI_REP_SNAPSHOT_ID     = "snapshot_id"
	CSI_REP_DATA_SOURCE     = "data_source"
	CSI_REP_CAPACITY_BYTES  = "capacity_bytes"
	CSI_VOLUME_ID_PREFIX    = "v1:"
	CSI_SNAPSHOT_ID_PREFIX  = "v1:"
	CSI_REQ_PREFIX          = "CSI_"
	CSI_REQ_SNAPSHOT_NAME   = CSI_REQ_PREFIX + "SNAPSHOT_NAME"
	CSI_REQ_SNAPSHOT_ID     = CSI_REQ_PREFIX + "SNAPSHOT_ID"
	CSI_REQ_VOLUME_ID       = CSI_REQ_PREFIX + "VOLUME_ID"
	CSI_REQ_DATA_SOURCE     = CSI_REQ_PREFIX + "DATA_SOURCE"
	CSI_REQ_SRC_SNAPSHOT_ID = CSI_REQ_PREFIX + "SRC_SNAPSHOT_ID"
	CSI_REQ_SRC_VOLUME_ID   = CSI_REQ_PREFIX + "SRC_VOLUME_ID"
	CSI_REQ_CAPACITY_BYTES  = CSI_REQ_PREFIX + "CAPACITY_BYTES"
	CSI_REQ_PARAM_PREFIX    = CSI_REQ_PREFIX + "PARAM_"
)
const (
	NFS_SHARE_SERVER_KEY = "nfs_server"
	NFS_SHARE_PATH_KEY   = "nfs_path"
)

type IdentityServer struct {
	csi.UnimplementedIdentityServer
}

func (d *IdentityServer) GetPluginInfo(ctx context.Context, req *csi.GetPluginInfoRequest) (*csi.GetPluginInfoResponse, error) {
	slog.Info("GetPluginInfo called")
	return &csi.GetPluginInfoResponse{
		Name:          DriverName,
		VendorVersion: DriverVersion,
	}, nil
}

func (d *IdentityServer) GetPluginCapabilities(ctx context.Context, req *csi.GetPluginCapabilitiesRequest) (*csi.GetPluginCapabilitiesResponse, error) {
	slog.Info("GetPluginCapabilities called")
	return &csi.GetPluginCapabilitiesResponse{
		Capabilities: []*csi.PluginCapability{
			{
				Type: &csi.PluginCapability_Service_{
					Service: &csi.PluginCapability_Service{
						Type: csi.PluginCapability_Service_CONTROLLER_SERVICE,
					},
				},
			},
		},
	}, nil
}
func (d *IdentityServer) Probe(ctx context.Context, req *csi.ProbeRequest) (*csi.ProbeResponse, error) {
	slog.Debug("Probe called")
	return &csi.ProbeResponse{}, nil
}

var _ csi.IdentityServer = &IdentityServer{}

type StringMutex struct {
	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

func NewStringMutex() *StringMutex {
	return &StringMutex{
		locks: make(map[string]*sync.Mutex),
	}
}

func (sm *StringMutex) TryLock(key string) bool {
	sm.mu.Lock()
	if sm.locks[key] == nil {
		sm.locks[key] = &sync.Mutex{}
	}
	lock := sm.locks[key]
	sm.mu.Unlock()
	return lock.TryLock()
}
func (sm *StringMutex) UnLock(key string) {
	sm.mu.Lock()
	if sm.locks[key] == nil {
		return
	}
	lock := sm.locks[key]
	sm.mu.Unlock()
	lock.Unlock()
}

type ExecFunc func() (err error)
type TimeoutFunc func() (err error)

func WaitUntilTimeout(timeout time.Duration, execFunc ExecFunc, timeoutFunc TimeoutFunc) error {
	// Create a channel to receive the result of the exec function
	done := make(chan bool, 1)
	var err error

	// Start the exec function in a goroutine
	go func() {
		err = execFunc()
		done <- true
	}()

	// Wait for the function to complete or time out
	select {
	case <-done:
		return err
	case <-time.After(timeout):
		return timeoutFunc()
	}
}
