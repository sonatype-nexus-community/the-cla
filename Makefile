.PHONY: all test build yarn air docker go-build go-alpine-build run-air
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test

AIRCMD=~/go/bin/air

all: test

docker:
	yarn version --patch
	docker build -t the-cla .
	docker image prune --force --filter label=stage=builder 

build: yarn go-build

yarn:
	yarn && yarn build

go-build:
	$(GOBUILD) -o the-cla ./server.go

go-alpine-build:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -o the-cla ./server.go

air: yarn
	$(GOBUILD) -o ./tmp/the-cla ./server.go

run-air: air
	docker run --name the_cla_postgres -p 5432:5432 -e POSTGRES_PASSWORD=the_cla -e POSTGRES_DB=db -d postgres
	$(AIRCMD) -c .air.toml && docker stop the_cla_postgres && docker rm the_cla_postgres

run-air-alone: yarn
	$(AIRCMD) -c .air.toml

test: build
	$(GOTEST) -v ./... 2>&1
