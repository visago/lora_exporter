.PHONY: all

REVISION          := $(shell git rev-parse HEAD)
VERSION          := $(shell git describe --tags --always --dirty="-dev")
BRANCH          := $(shell git rev-parse --abbrev-ref HEAD)
DATE             := $(shell date -u '+%Y-%m-%dT%H:%M:%S+00:00')
DAY             := $(shell date -u '+%Y.%m.%d')
VERSION_FLAGS    := -ldflags='-X "main.BuildVersion=$(VERSION)" -X "main.BuildRevision=$(REVISION)" -X "main.BuildTime=$(DATE)" -X "main.BuildBranch=$(BRANCH)"'

all:    lint build

lint:
	gofmt -w -s cmd/lora_exporter/*.go

build:
	CGO_ENABLED=0 go build ${VERSION_FLAGS} -o ./lora_exporter ./cmd/lora_exporter

build-docker:
	docker buildx build --push -t visago/lora_exporter:$(DAY) .
	docker buildx build --push -t visago/lora_exporter:latest .

run: lint
	DEBUG=1 LISTEN=0.0.0.0:5673 go run ${VERSION_FLAGS} ./cmd/lora_exporter
	
