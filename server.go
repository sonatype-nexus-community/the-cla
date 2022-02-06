//
// Copyright 2021-present Sonatype Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
//go:build go1.16
// +build go1.16

package main

import (
	"crypto/subtle"
	"database/sql"
	"fmt"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/sonatype-nexus-community/the-cla/buildversion"
	"github.com/sonatype-nexus-community/the-cla/db"
	ourGithub "github.com/sonatype-nexus-community/the-cla/github"
	"github.com/sonatype-nexus-community/the-cla/oauth"
	"github.com/sonatype-nexus-community/the-cla/types"

	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	webhook "gopkg.in/go-playground/webhooks.v5/github"
)

const defaultServicePort = ":4200"

const pathClaText string = "/cla-text"
const pathOAuthCallback string = "/oauth-callback"
const pathSignCla string = "/sign-cla"
const pathWebhook string = "/webhook-integration"
const pathInfo = "/info"
const pathSignature = "/signature"
const buildLocation string = "build"

const envReactAppClaVersion string = "REACT_APP_CLA_VERSION"
const envGhWebhookSecret string = "GH_WEBHOOK_SECRET"
const envReactAppGithubClientId string = "REACT_APP_GITHUB_CLIENT_ID"
const envGithubClientSecret string = "GITHUB_CLIENT_SECRET"

const msgUnhandledGitHubEventType = "I do not handle this type of event, sorry!"

var postgresDB db.IClaDB

var claCache = make(map[string]string)

const envPGHost = "PG_HOST"
const envPGPort = "PG_PORT"
const envPGUsername = "PG_USERNAME"
const envPGPassword = "PG_PASSWORD"
const envPGDBName = "PG_DB_NAME"
const envSSLMode = "SSL_MODE"
const envInfoUsername = "INFO_USERNAME"
const envInfoPassword = "INFO_PASSWORD"

var errRecovered error
var logger *zap.Logger

func main() {
	e := echo.New()

	var err error
	config := zap.NewProductionConfig()
	config.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
	logger, err = config.Build()
	if err != nil {
		e.Logger.Fatal("can not initialize zap logger: %+v", err)
	}
	defer func() {
		_ = logger.Sync()
	}()
	//e.Use(echozap.ZapLogger(logger))
	e.Use(ZapLoggerFilterAWS_ELB(logger))

	// NOTE: using middleware.Logger() makes lots of AWS ELB Healthcheck noise in server logs
	//e.Use(
	//	middleware.Logger(), // Log everything to stdout
	//)
	e.Debug = true

	defer func() {
		if r := recover(); r != nil {
			err, ok := r.(error)
			if !ok {
				err = fmt.Errorf("pkg: %v", r)
			}
			errRecovered = err
			logger.Error("panic", zap.Error(err))
		}
	}()

	buildInfoMessage := fmt.Sprintf("BuildVersion: %s, BuildTime: %s, BuildCommit: %s",
		buildversion.BuildVersion, buildversion.BuildTime, buildversion.BuildCommit)
	logger.Info("build", zap.String("buildMsg", buildInfoMessage))
	fmt.Println(buildInfoMessage)

	err = godotenv.Load(".env")
	if err != nil {
		logger.Error("env load", zap.Error(err))
	}

	pg, host, port, dbname, _, err := openDB()
	if err != nil {
		logger.Error("db open", zap.Error(err))
		panic(fmt.Errorf("failed to load database driver. host: %s, port: %d, dbname: %s, err: %+v", host, port, dbname, err))
	}
	defer func() {
		if err := pg.Close(); err != nil {
			logger.Error("db close", zap.Error(err))
		}
	}()

	err = pg.Ping()
	if err != nil {
		logger.Error("db ping", zap.Error(err))
		panic(fmt.Errorf("failed to ping database. host: %s, port: %d, dbname: %s, err: %+v", host, port, dbname, err))
	}

	postgresDB = db.New(pg, logger)

	err = postgresDB.MigrateDB("file://db/migrations")
	if err != nil {
		logger.Error("db migrate", zap.Error(err))
		panic(fmt.Errorf("failed to migrate database. err: %+v", err))
	} else {
		logger.Info("db migration complete")
	}

	e.Use(middleware.CORS())

	e.GET("/build-info", func(c echo.Context) error {
		return c.String(http.StatusOK, fmt.Sprintf("I am ALIVE. %s", buildInfoMessage))
	})

	e.GET(pathClaText, handleRetrieveCLAText)

	e.GET(pathOAuthCallback, handleProcessGitHubOAuth)

	e.POST(pathWebhook, handleProcessWebhook)

	e.PUT(pathSignCla, handleProcessSignCla)

	g := e.Group(pathInfo, middleware.BasicAuth(infoBasicValidator))
	g.GET(pathSignature, handleSignature)

	e.Static("/", buildLocation)

	routes := e.Routes()
	for _, v := range routes {
		routeInfo := fmt.Sprintf("%s %s as %s", v.Method, v.Path, v.Name)
		logger.Info("route", zap.String("info", routeInfo))
	}

	logger.Fatal("application end", zap.Error(e.Start(defaultServicePort)))
}

const queryParameterLogin = "login"
const queryParameterCLAVersion = "claversion"
const msgTemplateMissingQueryParam = "missing required query parameter: %s"
const hiddenFieldValue = "hidden"

//goland:noinspection GoUnusedParameter
func infoBasicValidator(username, password string, c echo.Context) (isValidLogin bool, err error) {
	// Be careful to use constant time comparison to prevent timing attacks
	if subtle.ConstantTimeCompare([]byte(username), []byte(os.Getenv(envInfoUsername))) == 1 &&
		subtle.ConstantTimeCompare([]byte(password), []byte(os.Getenv(envInfoPassword))) == 1 {
		isValidLogin = true
	} else {
		logger.Info("failed info endpoint login",
			zap.String("username", username),
			zap.String("password", password),
		)
	}
	return
}

func handleSignature(c echo.Context) (err error) {
	login, err := getRequiredQueryParameter(c, queryParameterLogin)
	if err != nil {
		return c.String(http.StatusUnprocessableEntity, err.Error())
	}

	claVersion, err := getRequiredQueryParameter(c, queryParameterCLAVersion)
	if err != nil {
		return c.String(http.StatusUnprocessableEntity, err.Error())
	}

	hasUserSignedCLA, foundUserSignature, err := postgresDB.HasAuthorSignedTheCla(login, claVersion)
	if err != nil {
		logger.Error("error checking signature", zap.Error(err))
		return c.String(http.StatusInternalServerError, err.Error())
	}
	if !hasUserSignedCLA {
		logger.Debug("cla not signed", zap.String("login", login))
		return c.String(http.StatusOK, fmt.Sprintf("cla version %s not signed by %s", claVersion, login))
	}

	// hide sensitive info
	foundUserSignature.User.Email = hiddenFieldValue
	foundUserSignature.User.GivenName = hiddenFieldValue
	logger.Debug("found login signature", zap.Any("foundUserSignature", foundUserSignature))
	return c.JSON(http.StatusOK, foundUserSignature)
}

func getRequiredQueryParameter(c echo.Context, parameterName string) (parameterValue string, err error) {
	parameterValue = c.QueryParam(parameterName)
	if parameterValue == "" {
		err = fmt.Errorf(msgTemplateMissingQueryParam, parameterName)
		logger.Error("invalid request", zap.Error(err))
		return
	}
	return
}

// ZapLoggerFilterAWS_ELB is a middleware and zap to provide an "access log" like logging for each request.
// Adapted from ZapLogger, until I find a better way to filter out AWS ELB Healthcheck messages.
func ZapLoggerFilterAWS_ELB(log *zap.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()

			err := next(c)
			if err != nil {
				c.Error(err)
			}

			req := c.Request()
			res := c.Response()

			fields := []zapcore.Field{
				zap.String("remote_ip", c.RealIP()),
				zap.String("latency", time.Since(start).String()),
				zap.String("host", req.Host),
				zap.String("request", fmt.Sprintf("%s %s", req.Method, req.RequestURI)),
				zap.Int("status", res.Status),
				zap.Int64("size", res.Size),
				zap.String("user_agent", req.UserAgent()),
			}

			userAgent := req.UserAgent()
			if strings.Contains(userAgent, "ELB-HealthChecker") {
				//fmt.Printf("userAgent: %s\n", userAgent)
				// skip logging of this AWS ELB healthcheck
				return nil
			}

			id := req.Header.Get(echo.HeaderXRequestID)
			if id == "" {
				id = res.Header().Get(echo.HeaderXRequestID)
				fields = append(fields, zap.String("request_id", id))
			}

			n := res.Status
			switch {
			case n >= 500:
				log.With(zap.Error(err)).Error("Server error", fields...)
			case n >= 400:
				log.With(zap.Error(err)).Warn("Client error", fields...)
			case n >= 300:
				log.Info("Redirection", fields...)
			default:
				log.Info("Success", fields...)
			}

			return nil
		}
	}
}

func openDB() (db *sql.DB, host string, port int, dbname, sslMode string, err error) {
	host = os.Getenv(envPGHost)
	port, _ = strconv.Atoi(os.Getenv(envPGPort))
	user := os.Getenv(envPGUsername)
	password := os.Getenv(envPGPassword)
	dbname = os.Getenv(envPGDBName)
	sslMode = os.Getenv(envSSLMode)

	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s "+
		"password=%s dbname=%s sslmode=%s",
		host, port, user, password, dbname, sslMode)
	db, err = sql.Open("postgres", psqlInfo)
	return
}

func handleProcessWebhook(c echo.Context) (err error) {
	callId := uuid.New()
	logger.Info("handleProcessWebhook-start",
		zap.Any("callId", callId),
	)
	defer func() {
		logger.Info("handleProcessWebhook-end",
			zap.Any("callId", callId),
		)
	}()

	ghSecret := os.Getenv(envGhWebhookSecret)

	hook, _ := webhook.New(webhook.Options.Secret(ghSecret))

	payload, err := hook.Parse(c.Request(), webhook.PullRequestEvent)

	if err != nil {
		if err == webhook.ErrEventNotFound {
			logger.Debug("Unsupported event type encountered", zap.Error(err))

			return c.String(http.StatusBadRequest, msgUnhandledGitHubEventType)
		}
		logger.Debug("error parsing pull request event", zap.Error(err))
		return c.String(http.StatusBadRequest, err.Error())
	}

	appId, err := ourGithub.GetAppId()
	if err != nil {
		return c.String(http.StatusBadRequest, err.Error())
	}

	switch payload := payload.(type) {
	case webhook.PullRequestPayload:
		switch payload.Action {
		case "opened", "reopened", "synchronize":
			err := ourGithub.HandlePullRequest(logger, postgresDB, payload, appId, getCurrentCLAVersion())
			if err != nil {
				logger.Error("failed to handle pull request", zap.Error(err))
				return c.String(http.StatusBadRequest, err.Error())
			}

			return c.String(http.StatusAccepted, "accepted pull request for processing")
		default:
			logger.Debug("ignore pull request payload",
				zap.String("action", payload.Action),
				zap.String("owner", payload.Repository.Owner.Login),
				zap.String("repo", payload.Repository.Name),
				zap.Int64("pullRequestID", payload.Number),
			)
			return c.String(http.StatusAccepted, fmt.Sprintf("No action taken for: %s", payload.Action))
		}
	default:
		// theoretically can't get here due to hook.Parse() call above (events param), but better safe than sorry
		logger.Debug("Unhandled payload type encountered", zap.Any("payload", payload))

		return c.String(http.StatusBadRequest, fmt.Sprintf("I do not handle this type of payload, sorry! Type: %T", payload))
	}
}

func getCurrentCLAVersion() (requiredClaVersion string) {
	// TODO should we read this from env var?
	return os.Getenv(envReactAppClaVersion)
}

func handleProcessSignCla(c echo.Context) (err error) {
	logger.Debug("Attempting to sign the CLA")
	user := new(types.UserSignature)

	if err := c.Bind(user); err != nil {
		return err
	}

	user.TimeSigned = time.Now()

	err = postgresDB.InsertSignature(user)
	if err != nil {
		logger.Error("failed to process sign cla", zap.Error(err))
		return c.String(http.StatusBadRequest, err.Error())
	}

	logger.Debug("CLA signed successfully")

	err = ourGithub.ReviewPriorPRs(logger, postgresDB, user)
	if err != nil {
		// log this, but don't fail the call
		logger.Error("error reviewing prior PRs", zap.Error(err))
	}

	return c.JSON(http.StatusCreated, user)
}

func handleProcessGitHubOAuth(c echo.Context) (err error) {
	logger.Debug("Attempting to fetch GitHub crud")

	code := c.QueryParam("code")

	state := c.QueryParam("state")
	if state == "" {
		return
	}

	oauthImpl := oauth.CreateOAuth(os.Getenv(envReactAppGithubClientId), os.Getenv(envGithubClientSecret))

	user, err := oauthImpl.GetOAuthUser(logger, code)
	if err != nil {
		logger.Error("failed to get oauth user", zap.Error(err))
		return
	}

	return c.JSON(http.StatusOK, user)
}

const envClsUrl = "CLA_URL"
const msgMissingClaUrl = "missing " + envClsUrl + " environment variable"

func handleRetrieveCLAText(c echo.Context) (err error) {
	logger.Debug("Attempting to fetch CLA text")
	claURL := os.Getenv(envClsUrl)

	if claCache[claURL] != "" {
		logger.Debug("CLA text was cached, returning", zap.String("claURL", claURL))

		return c.String(http.StatusOK, claCache[claURL])
	}

	logger.Debug("CLA text not in cache, moving forward to fetch", zap.String("claURL", claURL))
	if claURL == "" {
		return fmt.Errorf(msgMissingClaUrl)
	}

	client := http.Client{}

	resp, err := client.Get(claURL)
	if err != nil {
		logger.Error("failed to get cla url", zap.Error(err))
		return
	}

	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("unexpected cla text response code: %d", resp.StatusCode)
		logger.Error("failed to get cla text", zap.Error(err))
		return
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logger.Error("failed to read cla text", zap.Error(err))
		return
	}

	claCache[claURL] = string(content)

	return c.String(http.StatusOK, claCache[claURL])
}
