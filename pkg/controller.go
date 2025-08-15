package pkg

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"regexp"
	"strconv"
	"strings"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type ControllerCfg struct {
	Endpoint          string
	CreateCmd         string
	DeleteCmd         string
	ExpandCmd         string
	CreateSnapshotCmd string
	DeleteSnapshotCmd string
	SSHConfig         SshExecuter
}

type SshController struct {
	csi.UnimplementedControllerServer
	IdentityServer
	config   ControllerCfg
	executer Executer
	server   *GrpcServer
}

func NewController(config ControllerCfg) *SshController {
	server, err := NewGrpcServer(config.Endpoint)
	if err != nil {
		log.Fatalf("failed to create gRPC server: %v", err)
	}

	return &SshController{
		config:   config,
		server:   server,
		executer: &config.SSHConfig,
	}
}

var _ csi.ControllerServer = &SshController{}

func (d *SshController) Run() error {
	csi.RegisterIdentityServer(d.server.server, d)
	csi.RegisterControllerServer(d.server.server, d)

	slog.Info("Starting NFS Controller CSI driver", "name", DriverName, "version", DriverVersion, "endpoint", d.config.Endpoint)

	return d.server.Run()
}

type CreateVolumeShellResponse struct {
	VolumeID      string
	VolumePath    string
	CapacityBytes int64
}

var invalidEnvVarPattern = regexp.MustCompile(`[^a-zA-Z0-9]`)

func varToEnvName(name string) string {
	envKey := invalidEnvVarPattern.ReplaceAllString(name, "_")
	if len(envKey) > 60 {
		envKey = envKey[:60]
	}
	return envKey
}

func (d *SshController) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	slog.InfoContext(ctx, "CreateVolume called", "name", req.GetName(), "capacity", req.GetCapacityRange().GetRequiredBytes())
	volumeID := req.GetName()
	if volumeID == "" {
		return nil, status.Error(codes.InvalidArgument, "Volume name is required")
	}
	slog.Info("Creating volume", "volumeID", volumeID)
	env := map[string]string{
		CSI_REQ_VOLUME_ID:      volumeID,
		CSI_REQ_CAPACITY_BYTES: strconv.FormatInt(req.GetCapacityRange().GetRequiredBytes(), 10),
	}
	for k, v := range req.GetParameters() {
		env[CSI_REQ_PARAM_PREFIX+varToEnvName(k)] = v
	}
	if req.GetVolumeContentSource() != nil {
		vs := req.VolumeContentSource
		switch vs.Type.(type) {
		case *csi.VolumeContentSource_Snapshot:
			env[CSI_REQ_DATA_SOURCE] = "snapshot"
			snapshotID, err := trimSnapshotID(vs.GetSnapshot().GetSnapshotId())
			if err != nil {
				return nil, err
			}
			env[CSI_REQ_SRC_SNAPSHOT_ID] = snapshotID
		case *csi.VolumeContentSource_Volume:
			env[CSI_REQ_DATA_SOURCE] = "volume"
			volumeID, err := trimVolumeID(vs.GetVolume().GetVolumeId())
			if err != nil {
				return nil, err
			}
			env[CSI_REQ_SRC_VOLUME_ID] = volumeID
		default:
			return nil, status.Errorf(codes.InvalidArgument, "%v not a proper volume source", vs)
		}
	}
	slog.WarnContext(ctx, "Executing CreateVolume CMD", "req_id", volumeID)
	stdout, err := d.executer.ExecuteCommand(d.config.CreateCmd, env)

	if err != nil {
		slog.ErrorContext(ctx, "Create volume script failed", "id", volumeID, "err", err, "output", string(stdout))
		return nil, status.Errorf(codes.Internal, "Failed to create volume: %s", err)
	}
	shell_out, err := parseShellResponse(stdout)
	resVolumeID := PopKey(shell_out, CSI_REP_VOLUME_ID)
	if resVolumeID == "" {
		return nil, status.Errorf(codes.Internal, "Create script did not return volume_id")
	}
	slog.WarnContext(ctx, "Volume created successfully", "req_id", volumeID, "resp_id", resVolumeID)
	capacity_str := PopKey(shell_out, CSI_REP_CAPACITY_BYTES)
	capacity, err := strconv.ParseInt(capacity_str, 10, 64)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to parse capacity bytes", "id", volumeID, "err", err)
		return nil, status.Errorf(codes.Internal, "Failed to parse capacity bytes: %s", err)
	}

	serverName := PopKey(shell_out, NFS_SHARE_SERVER_KEY)
	serverPath := PopKey(shell_out, NFS_SHARE_PATH_KEY)
	if serverName == "" || serverPath == "" {
		return nil, status.Errorf(codes.Internal, "Create script did not return nfs share information")
	}
	contentSource := req.GetVolumeContentSource()
	if PopKey(shell_out, CSI_REP_DATA_SOURCE) == "" {
		contentSource = nil
	}

	return &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			VolumeId: CSI_VOLUME_ID_PREFIX + resVolumeID,
			VolumeContext: map[string]string{
				NFS_SHARE_SERVER_KEY: serverName,
				NFS_SHARE_PATH_KEY:   serverPath,
			},
			CapacityBytes: int64(capacity),
			ContentSource: contentSource,
		},
	}, nil
}

func PopKey(m map[string]string, key string) string {
	val, ok := m[key]
	if ok {
		delete(m, key)
	}
	return val
}

func parseShellResponse(stdout []byte) (map[string]string, error) {
	output := string(stdout)
	lines := strings.Split(output, "\n")
	resp := make(map[string]string)
	for _, line := range lines {
		if after, ok := strings.CutPrefix(line, CSI_SHELL_OUTPUT_PREFIX); ok {
			line = after
			vals := strings.SplitN(line, "=", 2)
			resp[vals[0]] = vals[1]
		}
	}
	return resp, nil
}

func (d *SshController) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	slog.InfoContext(ctx, "DeleteVolume called", "req_volume_id", req.GetVolumeId())
	volumeID, err := trimVolumeID(req.GetVolumeId())
	if err != nil {
		return nil, err
	}
	env := map[string]string{
		CSI_REQ_VOLUME_ID: volumeID,
	}
	slog.InfoContext(ctx, "Exec Deleting Volume CMD", "volumeID", volumeID)
	stdout, err := d.executer.ExecuteCommand(d.config.DeleteCmd, env)
	if err != nil {
		log.Printf("Delete volume script failed for %s: %s, output: %s", volumeID, err, string(stdout))
		return nil, status.Errorf(codes.Internal, "Failed to delete volume: %s", err)
	}
	slog.WarnContext(ctx, "Volume deleted successfully", "id", volumeID)
	return &csi.DeleteVolumeResponse{}, nil
}

func trimVolumeID(volumeID string) (string, error) {
	if after, ok := strings.CutPrefix(volumeID, CSI_VOLUME_ID_PREFIX); ok {
		if after == "" {
			return "", status.Error(codes.InvalidArgument, "Volume ID is empty")
		}
		return after, nil
	} else {
		return "", status.Error(codes.InvalidArgument, "Volume ID is invalid")
	}
}
func trimSnapshotID(snapshotID string) (string, error) {
	if after, ok := strings.CutPrefix(snapshotID, CSI_SNAPSHOT_ID_PREFIX); ok {
		if after == "" {
			return "", status.Error(codes.InvalidArgument, "Snapshot ID is empty")
		}
		return after, nil
	} else {
		return "", status.Error(codes.InvalidArgument, "Snapshot ID is invalid")
	}
}

func (d *SshController) ControllerExpandVolume(ctx context.Context, req *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	slog.InfoContext(ctx, "ControllerExpandVolume called", "req_volume_id", req.GetVolumeId())
	volumeID, err := trimVolumeID(req.GetVolumeId())
	if err != nil {
		return nil, err
	}
	env := map[string]string{
		CSI_REQ_VOLUME_ID:      volumeID,
		CSI_REQ_CAPACITY_BYTES: fmt.Sprintf("%d", req.GetCapacityRange().GetRequiredBytes()),
	}
	slog.InfoContext(ctx, "Exec Expanding Volume CMD", "volumeID", volumeID, "capacity", env[CSI_REQ_CAPACITY_BYTES])
	shell_out, err := d.execCmd(ctx, d.config.ExpandCmd, env)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to expand volume: %s", err)
	}
	capacity_str := PopKey(shell_out, CSI_REP_CAPACITY_BYTES)
	capacity, err := strconv.ParseInt(capacity_str, 10, 64)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to parse capacity bytes", "id", volumeID, "err", err)
		return nil, status.Errorf(codes.Internal, "Failed to parse capacity bytes: %s", err)
	}
	return &csi.ControllerExpandVolumeResponse{
		CapacityBytes: capacity,
	}, nil
}

func (d *SshController) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	slog.InfoContext(ctx, "ValidateVolumeCapabilities called", "volume_id", req.GetVolumeId())
	return &csi.ValidateVolumeCapabilitiesResponse{
		Confirmed: &csi.ValidateVolumeCapabilitiesResponse_Confirmed{
			VolumeCapabilities: req.GetVolumeCapabilities(),
		},
	}, nil
}

func (d *SshController) execCmd(ctx context.Context, cmd string, env map[string]string) (map[string]string, error) {
	stdout, err := d.executer.ExecuteCommand(cmd, env)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to execute command", "cmd", cmd, "err", err, "output", string(stdout))
		return nil, fmt.Errorf("failed to execute command %q: %w", cmd, err)
	}
	return parseShellResponse(stdout)
}

func popCapacityFromShellOutput(shell_out map[string]string) (int64, error) {
	capacityStr := PopKey(shell_out, CSI_REP_CAPACITY_BYTES)
	if capacityStr == "" {
		return 0, fmt.Errorf("capacity not found in shell output")
	}
	capacity, err := strconv.ParseInt(capacityStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse capacity: %w", err)
	}
	return capacity, nil
}

// create snapshot
func (d *SshController) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	slog.InfoContext(ctx, "CreateSnapshot called", "name", req.GetName(), "source_volume_id", req.GetSourceVolumeId())
	if d.config.CreateSnapshotCmd == "" {
		return nil, status.Error(codes.Unimplemented, "CreateSnapshot command is not configured")
	}
	if req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "CreateSnapshot name must be provided")
	}
	volumeID, err := trimVolumeID(req.GetSourceVolumeId())
	if err != nil {
		return nil, err
	}
	env := map[string]string{
		CSI_REQ_SNAPSHOT_NAME: req.GetName(),
		CSI_REQ_SRC_VOLUME_ID: volumeID,
	}
	result, err := d.execCmd(ctx, d.config.CreateSnapshotCmd, env)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to exec cmd %s", err)
	}
	snap_id := PopKey(result, CSI_REP_SNAPSHOT_ID)
	if snap_id == "" {
		return nil, status.Error(codes.Internal, "Failed to create snapshot: snapshot ID is empty")
	}
	capacity, err := popCapacityFromShellOutput(result)
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("Failed to parse snapshot capacity: %s", err))
	}

	return &csi.CreateSnapshotResponse{
		Snapshot: &csi.Snapshot{
			SnapshotId:     CSI_SNAPSHOT_ID_PREFIX + snap_id,
			SourceVolumeId: req.GetSourceVolumeId(),
			SizeBytes:      capacity,
			CreationTime:   timestamppb.Now(),
			ReadyToUse:     true,
		},
	}, nil
}

func (d *SshController) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	slog.InfoContext(ctx, "DeleteSnapshot called", "snapshot_id", req.GetSnapshotId())
	if d.config.DeleteSnapshotCmd == "" {
		return nil, status.Error(codes.Unimplemented, "DeleteSnapshot command is not configured")
	}
	snapshotID, err := trimSnapshotID(req.GetSnapshotId())
	if err != nil {
		return nil, err
	}
	env := map[string]string{
		CSI_REQ_SNAPSHOT_ID: snapshotID,
	}
	slog.WarnContext(ctx, "Exec Deleting Snapshot CMD", "id", snapshotID)
	result, err := d.execCmd(ctx, d.config.DeleteSnapshotCmd, env)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to exec cmd %s", err)
	}
	if PopKey(result, CSI_REP_SNAPSHOT_ID) != snapshotID {
		return nil, status.Error(codes.Internal, "Failed to delete snapshot: returned snapshot ID is empty or does not match requested ID")
	}
	return &csi.DeleteSnapshotResponse{}, nil
}

func (d *SshController) ControllerGetCapabilities(ctx context.Context, req *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
	slog.Info("ControllerGetCapabilities called")
	cap := &csi.ControllerGetCapabilitiesResponse{
		Capabilities: []*csi.ControllerServiceCapability{
			{
				Type: &csi.ControllerServiceCapability_Rpc{
					Rpc: &csi.ControllerServiceCapability_RPC{
						Type: csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
					},
				},
			},
			{
				Type: &csi.ControllerServiceCapability_Rpc{
					Rpc: &csi.ControllerServiceCapability_RPC{
						Type: csi.ControllerServiceCapability_RPC_CLONE_VOLUME,
					},
				},
			},
		},
	}
	if d.config.ExpandCmd != "" {
		cap.Capabilities = append(cap.Capabilities, &csi.ControllerServiceCapability{
			Type: &csi.ControllerServiceCapability_Rpc{
				Rpc: &csi.ControllerServiceCapability_RPC{
					Type: csi.ControllerServiceCapability_RPC_EXPAND_VOLUME,
				},
			},
		})
	}
	if d.config.CreateSnapshotCmd != "" {
		cap.Capabilities = append(cap.Capabilities, &csi.ControllerServiceCapability{
			Type: &csi.ControllerServiceCapability_Rpc{
				Rpc: &csi.ControllerServiceCapability_RPC{
					Type: csi.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT,
				},
			},
		})
	}
	return cap, nil
}
