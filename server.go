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
package main

import (
	"io/ioutil"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func main() {
	e := echo.New()
	addr := ":4200"

	err := godotenv.Load(".env")
	if err != nil {
		e.Logger.Fatal(err)
	}

	e.Use(middleware.CORS())

	e.GET("/cla-text", retrieveCLAText)

	e.Static("/", "build")

	e.Debug = true

	e.Logger.Fatal(e.Start(addr))
}

func retrieveCLAText(c echo.Context) (err error) {
	claURL := os.Getenv("CLA_URL")

	c.Logger().Debug(claURL)

	client := http.Client{}

	resp, err := client.Get(claURL)
	if err != nil {
		c.Logger().Error(err)
		return
	}

	c.Logger().Debug(resp.StatusCode)

	defer resp.Body.Close()

	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		c.Logger().Error(err)
		return
	}

	return c.String(http.StatusOK, string(content))
}
