.PHONY: build test test-race lint minio-up minio-test minio-down yandex-s3-smoke clean

GO ?= go
export GOCACHE ?= $(CURDIR)/.cache/go-build
export GOFLAGS ?= -buildvcs=false

build:
	mkdir -p bin
	$(GO) build -o bin/s3s5-client ./cmd/s3s5-client
	$(GO) build -o bin/s3s5-server ./cmd/s3s5-server
	$(GO) build -o bin/s3s5-doctor ./cmd/s3s5-doctor

test:
	$(GO) test ./...

test-race:
	$(GO) test -race ./...

lint:
	$(GO) vet ./...

minio-up:
	./scripts/minio-up.sh

minio-test:
	./scripts/minio-test.sh

minio-down:
	./scripts/minio-down.sh

yandex-s3-smoke:
	./scripts/yandex-s3-smoke.sh

clean:
	rm -rf bin coverage.out
