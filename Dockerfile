FROM golang:alpine AS builder
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 go build -o scrobble-exporter .

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /app/scrobble-exporter /scrobble-exporter
EXPOSE 9101
ENTRYPOINT ["/scrobble-exporter"]
