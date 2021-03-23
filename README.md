[![sonatype-nexus-community](https://circleci.com/gh/sonatype-nexus-community/the-cla.svg?style=shield)](https://circleci.com/gh/sonatype-nexus-community/the-cla)
# THE-CLA

The-Cla is an app written in Golang/React for getting CLA signatures. This is just a proof of concept at time being, use at your own risk.

## Development

### Setup

To get started with this project you will need:

- Golang (project started using Go 1.16.2, but likely anything above 1.14 is fine)
- Yarn/NodeJS
- Air

To install air:

- https://github.com/cosmtrek/air

You can run:

- `go get -u github.com/cosmtrek/air` in a folder outside of this project (so it is not added as a dependency)

### Running/Developing

Thanks to Air, there is some amount of "live-reload". To run the project, you can run `air -c .air.toml` in the project root. Once it is built, you should be able to access the site at `http://localhost:4200/`

Any code changes to golang/react files will cause a rebuild and restart, and will be accessible via the browser with a refresh!
