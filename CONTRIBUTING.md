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

To get started with this project you will need:

- Golang (project started using Go 1.16.2, but likely anything above 1.14 is fine)
- Yarn/NodeJS
- Air

To install air:

- https://github.com/cosmtrek/air

You can run:

- `go get -u github.com/cosmtrek/air` in a folder outside this project (so it is not added as a dependency).

  The `air` binary will be located in your `~/go/bin` folder, which may need to added to your commands and/or path.
  The [AIRCMD](Makefile#L6) setting in the Makefile may need to be adjusted if a different location is used.

### Running/Developing

Thanks to Air, there is some amount of "live-reload". To run the project, you can run `air -c .air.toml` in the project root. Once it is built, you should be able to access the site at `http://localhost:4200/`

Any code changes to golang/react files will cause a rebuild and restart, and will be accessible via the browser with a refresh!

For local development, a good first step is to copy the example `.env.example` file to `.env` and launch a local db
and `air` like so:
```shell
cp .example.env .env
make run-air
```

For some fun interactive debugging with [server.go](./server.go), you could spin up the local docker db image, and manually run
the server in debug more. See the [Makefile](./Makefile) for the latest and greatest commands to cherry-pick.
```shell
$ docker run --name the_cla_postgres -p 5432:5432 -e POSTGRES_PASSWORD=the_cla -e POSTGRES_DB=db -d postgres
34b413c68663b28d722fe2503b869a03bd2808e1facdcbbf5dde8a1ac0f6beb9...
```
Then run [server.go](./server.go) in debug mode in your favorite IDE, and enjoy break points activating when you connect to
endpoints. Wee!

For frontend work (with a previously manually launched database), this command is helpful for development:
```shell
make run-air-alone
```

#### Docker

Alternatively, if you just want to play around lightly, you can run the docker commands below. First set up
your environment as described in [App environment configuration](#app-environment-configuration), otherwise much may not
work, and you will miss out on much goodness.

- `make docker`
- `docker run -p 4200:4200 the-cla`

This will be a lot slower, but you can build and run the entire application with only `docker` (and `make`) installed, essentially.

### Coding Conventions

- In order to help verify the authenticity of contributed code, we ask that your [commits be signed](https://docs.github.com/en/authentication/managing-commit-signature-verification/signing-commits). 
  All commits must be signed off to show that you agree to publish your changes under the current terms and licenses of the project.
  
  Here are some notes we found helpful in configuring a local environment to automatically sign git commits:
    - [GPG commit signature verification](https://docs.github.com/en/authentication/managing-commit-signature-verification/about-commit-signature-verification#gpg-commit-signature-verification)
    - [Telling Git about your GPG key](https://docs.github.com/en/authentication/managing-commit-signature-verification/telling-git-about-your-signing-key#telling-git-about-your-gpg-key)

## Testing

You can run the Golang tests by running `make test`.

## Submitting Contributions

Please send Pull Requests that:
1. Have a singluar purpose, and that is backed by one or more GitHub Issues in this project
2. Are clear
3. Have appropriate test coverage for the Pull Requests purpose
4. Meet our Code Style Convention (see [above](#develpoment-guidelines))
