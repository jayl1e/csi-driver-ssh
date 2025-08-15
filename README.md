# csi-driver-ssh
csi driver via ssh shell command

## Purpose
- provide PV via shell script over ssh

## Usage
customize [deploy/manifest](https://github.com/jayl1e/csi-driver-ssh/tree/main/deploy/manifest) folder, usually you only edit to edit [controller plugin cmd arg](https://github.com/jayl1e/csi-driver-ssh/blob/main/deploy/manifest/plugin-controller.yaml#L128)

write your script for create volume, delete volume, create snapshot and so on

`kubectl apply -k .`

## Motivation
I can not find a PV/PVC solution for my kubernetes cluster. I need:
- central storage server, provide volume via net storage protocal like NFS
- quota limit
- snapshots

The [official NFS CSI driver](https://github.com/kubernetes-csi/csi-driver-nfs) can not satisfy my requirements because it simply create sub dir over NFS and it can not execute script like set quota.


## Next
- support script for node plugin, not only NFS
- support multiple server
