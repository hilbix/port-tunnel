#

love:
	go mod tidy
	go build -o port-tunnel ./cmd/port-tunnel
	./port-tunnel --config ./config.example.yaml --log-level=debug

porttunnel.exe:
	GOOS=windows GOARCH=amd64 go build -o $@ ./cmd/port-tunnel

porttunnel.arm64.linux:
	GOOS=linux GOARCH=arm64 go build -o $@ ./cmd/port-tunnel

