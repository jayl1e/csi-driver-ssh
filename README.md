# csi-driver-ssh
CSI driver via ssh shell command

*Welcome to contribute!*

## Usage
customize [deploy/manifest](https://github.com/jayl1e/csi-driver-ssh/tree/main/deploy/manifest) folder, usually you only edit to edit [controller plugin cmd arg](https://github.com/jayl1e/csi-driver-ssh/blob/main/deploy/manifest/plugin-controller.yaml#L128)

write your script for create volume, delete volume, create snapshot and so on

`kubectl apply -k .`

## Motivation
I can not find a PV/PVC solution for my kubernetes cluster. I need:
- central storage server, provide volume via net storage protocal like NFS
- quota limit
- snapshots
- simple

The exsiting alternatives have pitfalls:
- The [official NFS CSI driver](https://github.com/kubernetes-csi/csi-driver-nfs) can not satisfy my requirements because it simply create sub dir over NFS and it can not execute script like set quota.
- Vender specific CSI like [synology-csi](https://github.com/SynologyOpenSource/synology-csi) are limited to these specific vendors
- Network storages like Ceph are too heavy
- Local pv like [openebs](https://github.com/openebs)  are limited to node
- Simple network storage like [openebs](https://github.com/openebs) and [longhorn](https://github.com/longhorn/longhorn) are limited within one k8s cluster

## Features
- ssh
- shell hook
- you can set quota, create snapshot, clone volume as you wish via your custome script
- support btrfs, zfs, lvm, or basic dir over NFS

## Next
- add more test
- support script for node plugin, not only NFS
- support multiple servers
