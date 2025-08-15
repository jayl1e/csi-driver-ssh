src=/export/${CSI_VOLUME_ID}
target=/export/${CSI_SNAPSHOT_NAME}.tar.gz
snapshot_id=${CSI_SNAPSHOT_NAME}
echo tar -czvf $target $src
echo "csi-shell-output:snapshot_id=${snapshot_id}"
echo "csi-shell-output:capacity_bytes=1"