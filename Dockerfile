#
# Copyright (c) 2021-present Sonatype, Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

FROM node:18-alpine3.18 AS yarn-build
ARG REACT_APP_CLA_URL=http://something
ARG REACT_APP_COMPANY_NAME=A Company Name Here
ARG REACT_APP_COMPANY_WEBSITE=http://localhost
ARG REACT_APP_CLA_APP_NAME=CLA App
ARG REACT_APP_GITHUB_CLIENT_ID=FAKE_ID
ARG REACT_APP_CLA_VERSION=0.0
LABEL stage=builder

RUN apk add --no-cache build-base

WORKDIR /src

COPY . .

RUN REACT_APP_CLA_URL="$REACT_APP_CLA_URL" \
    REACT_APP_COMPANY_NAME="$REACT_APP_COMPANY_NAME" \
    REACT_APP_COMPANY_WEBSITE="$REACT_APP_COMPANY_WEBSITE" \
    REACT_APP_CLA_APP_NAME="$REACT_APP_CLA_APP_NAME" \
    REACT_APP_GITHUB_CLIENT_ID="$REACT_APP_GITHUB_CLIENT_ID" \
    REACT_APP_CLA_VERSION="$REACT_APP_CLA_VERSION" \
    make yarn

FROM golang:1.23-alpine AS build
LABEL stage=builder

RUN apk add --no-cache build-base ca-certificates git

ENV USER=clauser
ENV UID=10001 

WORKDIR /src

RUN adduser \    
    --disabled-password \    
    --gecos "" \    
    --home "/nonexistent" \    
    --shell "/sbin/nologin" \    
    --no-create-home \    
    --uid "${UID}" \    
    "${USER}"

COPY . .

# Ensures that the build from yarn is used, not an existing build on the local device
COPY --from=yarn-build /src/build /src/build

RUN make go-alpine-build

FROM scratch AS bin
LABEL application=the-cla

EXPOSE 4200

WORKDIR /

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /etc/passwd /etc/passwd
COPY --from=build /etc/group /etc/group
COPY --from=build /src/build /build
COPY --from=build /src/the-cla /
COPY *.env /
COPY *the-cla.pem /
COPY db/migrations /db/migrations

USER clauser:clauser

ENTRYPOINT [ "./the-cla" ]
