module github.com/sonatype-nexus-community/the-cla

go 1.16

require (
	github.com/bradleyfalzon/ghinstallation v1.1.1 // indirect
	github.com/google/go-github/v33 v33.0.0
	github.com/joho/godotenv v1.3.0
	github.com/labstack/echo/v4 v4.2.1
	github.com/stretchr/testify v1.7.0
	golang.org/x/oauth2 v0.0.0-20210402161424-2e8d93401602
	gopkg.in/go-playground/webhooks.v5 v5.17.0
)

replace github.com/yuin/goldmark => github.com/yuin/goldmark v1.2.0
