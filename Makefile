.PHONY: build test test-race lint release-artifacts perf-test perf-test-simulated perf-report perf-baseline android-build android-test android-docker-image android-docker-build android-docker-test server-package-image server-package-deb server-package-rpm server-package server-package-arm64 server-package-all-arch server-package-deb-all-arch server-package-rpm-all-arch minio-up minio-test minio-down yandex-s3-smoke clean

GO ?= go
export GOCACHE ?= $(CURDIR)/.cache/go-build
export GOFLAGS ?= -buildvcs=false
VERSION ?= $(shell cat VERSION)
COMMIT ?= $(shell git rev-parse --short=12 HEAD 2>/dev/null || echo unknown)
DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w -X s3s5/internal/version.Version=$(VERSION) -X s3s5/internal/version.Commit=$(COMMIT) -X s3s5/internal/version.Date=$(DATE)

build:
	mkdir -p bin
	$(GO) build -ldflags "$(LDFLAGS)" -o bin/s3s5-client ./cmd/s3s5-client
	$(GO) build -ldflags "$(LDFLAGS)" -o bin/s3s5-server ./cmd/s3s5-server
	$(GO) build -ldflags "$(LDFLAGS)" -o bin/s3s5-doctor ./cmd/s3s5-doctor
	$(GO) build -ldflags "$(LDFLAGS)" -o bin/s3s5-perf ./cmd/s3s5-perf

test:
	$(GO) test ./...

test-race:
	$(GO) test -race ./...

lint:
	$(GO) vet ./...

release-artifacts:
	./scripts/build-release-artifacts.sh

perf-test:
	$(GO) run ./cmd/s3s5-perf run -profile memory -out benchmarks/results/local/perf-memory.json

perf-test-simulated:
	$(GO) run ./cmd/s3s5-perf run -profile simulated-s3 -out benchmarks/results/local/perf-simulated-s3.json -short-connections 3 -idle-sessions 3 -idle-duration 50ms -chatty-duration 50ms -put-delay 10ms -get-delay 10ms -head-delay 10ms -list-delay 12ms -delete-delay 10ms -jitter 1ms

perf-report:
	$(GO) run ./cmd/s3s5-perf report -in $${PERF_JSON:-benchmarks/results/baseline-v1-memory.json} -out $${PERF_REPORT:-benchmarks/reports/baseline-v1.md}

perf-baseline:
	$(GO) run ./cmd/s3s5-perf baseline

android-build:
	cd android-client && ./gradlew :app:assembleDebug

android-test:
	cd android-client && ./gradlew :app:testDebugUnitTest

android-docker-image:
	./android-client/scripts/docker-build-image.sh

android-docker-build:
	./android-client/scripts/docker-gradle.sh :app:assembleDebug

android-docker-test:
	./android-client/scripts/docker-gradle.sh :app:testDebugUnitTest

server-package-image:
	S3S5_PACKAGE_DOCKER_REBUILD=1 ./scripts/package-server-docker.sh --image-only

server-package-deb:
	./scripts/package-server-docker.sh deb

server-package-rpm:
	./scripts/package-server-docker.sh rpm

server-package:
	./scripts/package-server-docker.sh all

server-package-arm64:
	GOARCH=arm64 ./scripts/package-server-docker.sh all

server-package-all-arch:
	./scripts/package-server-all-arch.sh all

server-package-deb-all-arch:
	./scripts/package-server-all-arch.sh deb

server-package-rpm-all-arch:
	./scripts/package-server-all-arch.sh rpm

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
