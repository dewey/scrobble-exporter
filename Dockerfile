FROM golang:alpine AS builder
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 go build -o scrobble-exporter .

FROM busybox:stable-musl AS busybox

FROM scratch
COPY --from=busybox /bin/wget /bin/wget
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /app/scrobble-exporter /scrobble-exporter
EXPOSE 9101
ENTRYPOINT ["/scrobble-exporter"]
