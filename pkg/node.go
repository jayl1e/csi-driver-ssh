package pkg

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	mount "k8s.io/mount-utils"
)

type NodeCfg struct {
	Endpoint        string
	MountPermission uint64
	NodeID          string
}

type SshNodeServer struct {
	csi.UnimplementedNodeServer
	IdentityServer
	config  NodeCfg
	mounter mount.Interface
	server  *GrpcServer
	mutex   *StringMutex
}

func NewNodeServer(config NodeCfg) *SshNodeServer {
	server, err := NewGrpcServer(config.Endpoint)
	if err != nil {
		log.Fatalf("failed to create gRPC server: %v", err)
	}
	mounter := mount.New("")
	if runtime.GOOS == "linux" {
		// MounterForceUnmounter is only implemented on Linux now
		mounter = mounter.(mount.MounterForceUnmounter)
	}
	return &SshNodeServer{
		config:  config,
		server:  server,
		mounter: mounter,
		mutex:   NewStringMutex(),
	}
}

func (d *SshNodeServer) Run() error {
	csi.RegisterIdentityServer(d.server.server, d)
	csi.RegisterNodeServer(d.server.server, d)

	slog.Info("Starting SSH Node CSI driver", "name", DriverName, "version", DriverVersion, "endpoint", d.config.Endpoint)
	return d.server.Run()
}

func (d *SshNodeServer) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	slog.Info("NodePublishVolume called", "volume_id", req.GetVolumeId())
	volCap := req.GetVolumeCapability()
	if volCap == nil {
		return nil, status.Error(codes.InvalidArgument, "Volume capability missing in request")
	}
	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}
	targetPath := req.GetTargetPath()
	if len(targetPath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target path not provided")
	}

	mutexKey := fmt.Sprintf("%s-%s", volumeID, targetPath)
	if acquired := d.mutex.TryLock(mutexKey); !acquired {
		return nil, status.Errorf(codes.Aborted, "volume operation already exists: %s", volumeID)
	}
	defer d.mutex.UnLock(mutexKey)

	mountOptions := volCap.GetMount().GetMountFlags()
	if req.GetReadonly() {
		mountOptions = append(mountOptions, "ro")
	}

	mountPermission := d.config.MountPermission

	params := req.GetVolumeContext()
	nfsServer := params[NFS_SHARE_SERVER_KEY]
	nfsPath := params[NFS_SHARE_PATH_KEY]

	if nfsServer == "" {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("%v is a required parameter", NFS_SHARE_SERVER_KEY))
	}
	if nfsPath == "" {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("%v is a required parameter", NFS_SHARE_PATH_KEY))
	}
	source := fmt.Sprintf("%s:%s", nfsServer, nfsPath)

	notMnt, err := d.mounter.IsLikelyNotMountPoint(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(targetPath, os.FileMode(mountPermission)); err != nil {
				return nil, status.Error(codes.Internal, err.Error())
			}
			notMnt = true
		} else {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	if !notMnt {
		return &csi.NodePublishVolumeResponse{}, nil
	}

	slog.InfoContext(ctx, "NodePublishVolume", "volumeID", volumeID, "source", source, "targetPath", targetPath, "mountflags", mountOptions)
	execFunc := func() error {
		return d.mounter.Mount(source, targetPath, "nfs", mountOptions)
	}
	timeoutFunc := func() error { return fmt.Errorf("time out") }
	if err := WaitUntilTimeout(90*time.Second, execFunc, timeoutFunc); err != nil {
		if os.IsPermission(err) {
			return nil, status.Error(codes.PermissionDenied, err.Error())
		}
		if strings.Contains(err.Error(), "invalid argument") {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	if mountPermission > 0 {
		if err := chmodIfPermissionMismatch(targetPath, os.FileMode(mountPermission)); err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
	} else {
		slog.WarnContext(ctx, "skip chmod on targetPath", "targetPath", targetPath, "mountPermissions", mountPermission)
	}
	slog.InfoContext(ctx, "volume mount succeeded", "volumeID", volumeID, "source", source, "targetPath", targetPath)
	return &csi.NodePublishVolumeResponse{}, nil
}

func (d *SshNodeServer) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	slog.Info("NodeUnpublishVolume called", "volume_id", req.GetVolumeId())
	volumeID := req.GetVolumeId()
	if volumeID == "" {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}
	targetPath := req.GetTargetPath()
	if targetPath == "" {
		return nil, status.Error(codes.InvalidArgument, "Target path missing in request")
	}

	mutexKey := fmt.Sprintf("%s-%s", volumeID, targetPath)
	if acquired := d.mutex.TryLock(mutexKey); !acquired {
		return nil, status.Errorf(codes.Aborted, "volume operation already exists: %s", volumeID)
	}
	defer d.mutex.UnLock(mutexKey)

	slog.InfoContext(ctx, "NodeUnpublishVolume: unmounting volume", "volumeID", volumeID, "targetPath", targetPath)
	var err error
	extensiveMountPointCheck := true
	forceUnmounter, ok := d.mounter.(mount.MounterForceUnmounter)
	if ok {
		slog.Info("force unmount", "volumeID", volumeID, "targetPath", targetPath)
		err = mount.CleanupMountWithForce(targetPath, forceUnmounter, extensiveMountPointCheck, 30*time.Second)
	} else {
		err = mount.CleanupMountPoint(targetPath, d.mounter, extensiveMountPointCheck)
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to unmount target %q: %v", targetPath, err)
	}
	slog.Info("NodeUnpublishVolume: unmount volume", "volumeID", volumeID, "targetPath", targetPath)

	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (d *SshNodeServer) NodeGetCapabilities(ctx context.Context, req *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	slog.Info("NodeGetCapabilities called")
	return &csi.NodeGetCapabilitiesResponse{
		Capabilities: []*csi.NodeServiceCapability{
			{
				Type: &csi.NodeServiceCapability_Rpc{
					Rpc: &csi.NodeServiceCapability_RPC{
						Type: csi.NodeServiceCapability_RPC_SINGLE_NODE_MULTI_WRITER,
					},
				},
			},
			{
				Type: &csi.NodeServiceCapability_Rpc{
					Rpc: &csi.NodeServiceCapability_RPC{
						Type: csi.NodeServiceCapability_RPC_UNKNOWN,
					},
				},
			},
		},
	}, nil
}

func (d *SshNodeServer) NodeGetInfo(_ context.Context, _ *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	return &csi.NodeGetInfoResponse{
		NodeId: d.config.NodeID,
	}, nil
}

var _ csi.NodeServer = &SshNodeServer{}

func chmodIfPermissionMismatch(target string, mode os.FileMode) error {
	info, err := os.Lstat(target)
	if err != nil {
		return err
	}
	perm := info.Mode() & os.ModePerm
	if perm != mode {
		slog.Info("chmod", "targetPath", target, "mode", fmt.Sprintf("0%o", mode), "permissions", fmt.Sprintf("0%o", info.Mode()))
		if err := os.Chmod(target, mode); err != nil {
			return err
		}
	} else {
		slog.Info("skip chmod", "targetPath", target, "permissions", fmt.Sprintf("0%o", info.Mode()))
	}
	return nil
}
