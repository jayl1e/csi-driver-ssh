# https://github.com/kubernetes-csi/external-snapshotter
kubectl kustomize https://github.com/kubernetes-csi/external-snapshotter/client/config/crd | kubectl apply -f -
kubectl -n kube-system kustomize https://github.com/kubernetes-csi/external-snapshotter/deploy/kubernetes/snapshot-controller| kubectl apply -f -
kubectl -n kube-system scale deployment  snapshot-controller --replicas 1
