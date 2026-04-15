.PHONY: build test vet

BINARY = k8s-crd-lsp

build:
	go build -o $(BINARY) ./cmd/k8s-crd-lsp

test:
	go test ./...

vet:
	go vet ./...

release-local:
	GOOS=darwin GOARCH=arm64 go build -o dist/$(BINARY)-darwin-arm64 ./cmd/k8s-crd-lsp
	GOOS=darwin GOARCH=amd64 go build -o dist/$(BINARY)-darwin-amd64 ./cmd/k8s-crd-lsp
	GOOS=linux GOARCH=arm64 go build -o dist/$(BINARY)-linux-arm64 ./cmd/k8s-crd-lsp
	GOOS=linux GOARCH=amd64 go build -o dist/$(BINARY)-linux-amd64 ./cmd/k8s-crd-lsp
