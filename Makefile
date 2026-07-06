.PHONY: build test test-race lint android-build android-test android-docker-image android-docker-build android-docker-test server-package-image server-package-deb server-package-rpm server-package minio-up minio-test minio-down yandex-s3-smoke clean

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
