.PHONY: build test vet fmt

build:
	CGO_ENABLED=0 go build -trimpath -o kynotes-server ./cmd/kynotes-server

test:
	go test -race ./...

vet:
	go vet ./...

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './vendor/*')
