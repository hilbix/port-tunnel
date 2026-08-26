#

love:
	go mod tidy
	go build -o port-tunnel ./cmd/port-tunnel
	./port-tunnel --config ./config.example.yaml --log-level=debug

