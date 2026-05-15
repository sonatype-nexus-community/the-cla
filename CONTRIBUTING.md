# How to contribute

It's great you're here and reading this guide, because we need volunteers to help keep this project active and alive for the greater benefit of everyone!

- [Engaging with this project](#engaging-with-this-project)
- [Development Guidelines](#development-guidelines)
  - [Setup](#setup)
  - [Running/Developing](#runningdeveloping)
  - [Coding Conventions](#coding-conventions)
- [Testing](#testing)
- [Submitting Contributions](#submitting-contributions)

## Engaging with this project

Here are some important resources:
- [GitHub Issues](https://github.com/sonatype-nexus-community/the-cla/issues) - a place for bugs to be raised and feature requests made
- [GitHub Discussions](https://github.com/sonatype-nexus-community/the-cla/discussions) - a place to discuss ideas or real-world usage

## Development Guidelines

### Setup

Prerequisites:

- [Go 1.25+](https://go.dev/dl/)
- [Node.js 20+](https://nodejs.org/)
- [Docker](https://www.docker.com/) (for running PostgreSQL locally)

Install dependencies:

```shell
npm install       # frontend
go mod download   # backend
```

### Running/Developing

Copy the example env file and fill in your values (see [App Environment Configuration](README.md#app-environment-configuration)):

```shell
cp .example.env .env
```

Start a local PostgreSQL instance:

```shell
docker run --name the_cla_postgres -p 55432:5432 \
  -e POSTGRES_PASSWORD=the_cla -e POSTGRES_DB=db -d postgres
```

Start the backend:

```shell
go run ./server.go
```

Start the frontend dev server (proxies API calls to port 4200):

```shell
npm start
```

The app will be available at `http://localhost:3000/`. For backend debugging, run `server.go` in your IDE's debugger with the same environment variables.

### Coding Conventions

- In order to help verify the authenticity of contributed code, we ask that your [commits be signed](https://docs.github.com/en/authentication/managing-commit-signature-verification/signing-commits). 
  All commits must be signed off to show that you agree to publish your changes under the current terms and licenses of the project.
  
  Here are some notes we found helpful in configuring a local environment to automatically sign git commits:
    - [GPG commit signature verification](https://docs.github.com/en/authentication/managing-commit-signature-verification/about-commit-signature-verification#gpg-commit-signature-verification)
    - [Telling Git about your GPG key](https://docs.github.com/en/authentication/managing-commit-signature-verification/telling-git-about-your-signing-key#telling-git-about-your-gpg-key)

## Testing

```shell
go test ./...     # backend
npm test          # frontend
```

## Submitting Contributions

Please send Pull Requests that:
1. Have a singluar purpose, and that is backed by one or more GitHub Issues in this project
2. Are clear
3. Have appropriate test coverage for the Pull Requests purpose
4. Meet our Code Style Convention (see [above](#development-guidelines))
