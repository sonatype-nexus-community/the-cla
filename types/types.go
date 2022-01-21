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
// +build go1.16

package types

import "time"

type User struct {
	Login     string `json:"login"`
	Email     string `json:"email"`
	GivenName string `json:"name"`
}

type UserSignature struct {
	User       User   `json:"user"`
	CLAVersion string `json:"claVersion"`
	TimeSigned time.Time
}
