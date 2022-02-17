.PHONY: all test build yarn air docker go-build go-alpine-build run-air run-air-alone
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test

AIRCMD=~/go/bin/air

TAG_COMMIT := $(shell git rev-list --abbrev-commit --tags --max-count=1)
TAG := $(shell git describe --abbrev=0 --tags ${TAG_COMMIT} 2>/dev/null || true)
COMMIT := $(shell git rev-parse --short HEAD)
DATE := $(shell git log -1 --format=%cd --date=iso)
VERSION := $(TAG:v%=%)
ifneq ($(COMMIT), $(TAG_COMMIT))
	VERSION := 0.0.0-dev
endif
ifeq ($(VERSION),)
	VERSION := $(COMMIT)-$(DATE)
endif
ifneq ($(shell git status --porcelain),)
	VERSION := $(VERSION)-dirty
endif
GOBUILD_FLAGS=-ldflags="-X 'github.com/sonatype-nexus-community/the-cla/buildversion.BuildVersion=$(VERSION)' \
	   -X 'github.com/sonatype-nexus-community/the-cla/buildversion.BuildTime=$(DATE)' \
	   -X 'github.com/sonatype-nexus-community/the-cla/buildversion.BuildCommit=$(COMMIT)'"

all: test

docker:
	yarn version --patch
	docker build -t the-cla .
	docker image prune --force --filter label=stage=builder 

build: yarn go-build

yarn:
	yarn && yarn build

go-build:
	echo "VERSION: $(VERSION)"
	echo "DATE: $(DATE)"
	echo "COMMIT: $(COMMIT)"
	$(GOBUILD) -o the-cla $(GOBUILD_FLAGS) ./server.go

go-alpine-build:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -o the-cla $(GOBUILD_FLAGS) ./server.go

air: yarn
	$(GOBUILD) -o ./tmp/the-cla $(GOBUILD_FLAGS) ./server.go

run-air: air
	docker run --name the_cla_postgres -p 5432:5432 -e POSTGRES_PASSWORD=the_cla -e POSTGRES_DB=db -d postgres
	$(AIRCMD) -c .air.toml && docker stop the_cla_postgres && docker rm the_cla_postgres

run-air-alone: yarn
	$(AIRCMD) -c .air.toml

test: build
	$(GOTEST) -v ./... 2>&1
