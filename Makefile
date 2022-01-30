.DEFAULT_GOAL := build

snapshot:
	goreleaser release --snapshot --rm-dist

build:
	CGO_ENABLED=0 go build -o bin/vault-plugin-tailscale main.go

vault-test:
	vault server -dev -dev-root-token-id=root -dev-plugin-dir=./bin

test:
	go test -race ./...
