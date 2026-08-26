#

love:
	go mod tidy
	go build -o port-tunnel ./cmd/port-tunnel
	./port-tunnel --config /etc/port-tunnel/config.yaml --log-level=debug

