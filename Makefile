.PHONY: all

all:
	yarn && yarn build
	go build -o the-cla ./server.go

air:
	yarn && yarn build
	go build -o ./tmp/the-cla ./server.go
