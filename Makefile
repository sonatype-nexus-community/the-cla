.PHONY: all test build air
GOCMD=go
GOTEST=$(GOCMD) test

all: test

build:
	yarn && yarn build
	go build -o the-cla ./server.go

air:
	yarn && yarn build
	go build -o ./tmp/the-cla ./server.go

test: build
	$(GOTEST) -v ./... 2>&1
