module github.com/sonatype-nexus-community/the-cla

go 1.16

require (
	github.com/bradleyfalzon/ghinstallation v1.1.1
	github.com/golang-migrate/migrate/v4 v4.14.1
	github.com/google/go-github/v33 v33.0.0
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/joho/godotenv v1.3.0
	github.com/labstack/echo/v4 v4.2.1
	github.com/lib/pq v1.10.0 // indirect
	github.com/stretchr/testify v1.7.0
	golang.org/x/oauth2 v0.0.0-20210402161424-2e8d93401602
	gopkg.in/go-playground/webhooks.v5 v5.17.0
)

replace github.com/dhui/dktest => github.com/dhui/dktest v0.3.4
