run:
	go run cmd/controller/main.go
test:
	go test ./...
bin/csi-controller: cmd/controller/ pkg/
	go build -o $@ ./cmd/controller/
bin/csi-node: cmd/node/ pkg/
	go build -o $@ ./cmd/node/
build: bin/csi-controller bin/csi-node
docker:
	docker build -t csi-driver-ssh .
docker-build-push:
	docker buildx build --platform=linux/amd64,linux/arm64 -t jayl1e/csi-driver-ssh . --push
.PHONY: test run build docker
