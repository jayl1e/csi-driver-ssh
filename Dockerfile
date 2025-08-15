FROM golang AS builder
WORKDIR /app
COPY . .
RUN make build

FROM debian
RUN apt update && apt-get install -y nfs-common ca-certificates mount
COPY --from=builder /app/bin/* /bin/
