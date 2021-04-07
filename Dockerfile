#
# Copyright 2021-present Sonatype Inc.
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
FROM node:15.12.0-alpine3.13 as yarn-build
LABEL stage=builder

RUN apk add --update build-base

WORKDIR /src

COPY . .

RUN make yarn

FROM golang:1.16.0-alpine AS build
LABEL stage=builder

RUN apk add --update build-base ca-certificates

ENV USER=clauser
ENV UID=10001 

WORKDIR /src

COPY --from=yarn-build /src/build /src/build

RUN adduser \    
    --disabled-password \    
    --gecos "" \    
    --home "/nonexistent" \    
    --shell "/sbin/nologin" \    
    --no-create-home \    
    --uid "${UID}" \    
    "${USER}"

COPY . .

RUN make go-alpine-build

RUN ls

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
COPY the-cla.pem /

USER clauser:clauser

ENTRYPOINT [ "./the-cla" ]