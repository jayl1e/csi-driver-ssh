target=/export/${CSI_VOLUME_ID}
if [[ "$CSI_VOLUME_ID" != "test-volume" ]]; then
  exit 1
fi
echo "csi-shell-output:capacity_bytes=${CSI_CAPACITY_BYTES}"
