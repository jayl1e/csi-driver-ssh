target=/export/${CSI_VOLUME_ID}
echo mkdir -p $target
echo "csi-shell-output:volume_id=${CSI_VOLUME_ID}"
echo "csi-shell-output:capacity_bytes=${CSI_CAPACITY_BYTES}"
echo "csi-shell-output:nfs_path=/export/${CSI_VOLUME_ID}"
echo "csi-shell-output:nfs_server=localhost"