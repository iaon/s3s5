FROM golang:1.24-bookworm

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        build-essential \
        ca-certificates \
        dpkg-dev \
        file \
        rpm \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /work
