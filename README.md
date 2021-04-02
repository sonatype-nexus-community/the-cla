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

#### Docker

Alternatively, if you just want to play around lightly, you can run:

- `make docker`
- `docker run -p 4200:4200 the-cla`

This will be a lot slower, but you can build and run the entire application with only `docker` (and `make`) installed, essentially.

## Deployment

Thankfully, we've made this as simple as possible, we think? It'll get simpler with time, I'm sure :)

You will need:

- `terraform`
- `aws cli`
- `aws-vault`
- `docker`

### Terraform

- `aws-vault exec <your_profile> --backened=keychain terraform init`
- `aws-vault exec <your_profile> --backened=keychain terraform apply`

This should create all the nice lil AWS resources to manage this application, using ECS and ECR!

### Docker

To create the docker image:

- `make docker`

### Deployment

An executable bash script similar to the following will make pushing images easier:

```bash
#!/bin/bash
aws-vault exec <your_profile> --backend=keychain aws ecr get-login-password --region <aws_region> | docker login --username AWS --password-stdin <aws_account_id>.dkr.ecr.<aws_region>.amazonaws.com
docker tag the-cla:latest <aws_account_id>.dkr.ecr.<aws_region>.amazonaws.com/the-cla-app:latest
docker push <aws_account_id>.dkr.ecr.<aws_region>.amazonaws.com/the-cla-app:latest
```

Replace the stuff in the `<>` with your values (and remove the `<>` characters if that isn't immediately apparent), `chmod +x docker.sh`, and `./docker.sh`

After you have done this, you SHOULD have a running service, somewhere in AWS :)

### GitHub

More to come! This is where we will explain how to setup the oauth app!
