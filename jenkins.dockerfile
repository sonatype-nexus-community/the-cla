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

FROM docker-all.repo.sonatype.com/cdi/golang-1.17.1:2

RUN apt-get update && apt-get install -y curl

# See https://github.com/nodesource/distributions/blob/master/README.md#debinstall
RUN curl -fsSL https://deb.nodesource.com/setup_16.x | bash -

RUN apt-get update && apt-get install -y nodejs
RUN npm install --global yarn
ENV GOPATH=

USER jenkins
RUN go install github.com/sonatype-nexus-community/nancy@latest

#  root dir mounted as workspace. instead, for local testing, use: docker run -it -v $(pwd):/ws ...
#COPY . .
