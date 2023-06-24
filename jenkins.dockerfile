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
RUN curl -fsSL https://deb.nodesource.com/setup_18.x | bash -

RUN apt-get update && apt-get install -y nodejs
RUN npm install --global yarn
ENV GOPATH=

# install nancy so we can run scans
USER jenkins
#RUN go install github.com/sonatype-nexus-community/nancy@latest
# Goofy attempt to get around the new 'go install'/w/'replace' issue
#RUN mkdir tools
#RUN cd tools && git clone https://github.com/sonatype-nexus-community/nancy.git && cd nancy && go install
# Slightly less goofy? Install prebuild binary.
RUN  cd /tmp && mkdir tools && cd - && \
     latest_version_is=$(curl --fail -s https://api.github.com/repos/sonatype-nexus-community/nancy/releases/latest | grep -oP '"tag_name": "\K(.*)(?=")') && \
     desiredVersion=${latest_version_is} && \
     sourceUrl="https://github.com/sonatype-nexus-community/nancy/releases/download/${desiredVersion}/nancy-${desiredVersion}-linux-amd64" && \
     curl --fail -s -L "$sourceUrl" -o "/tmp/tools/nancy" && \
     chmod +x /tmp/tools/nancy

#  root dir mounted as workspace. instead, for local testing, use: docker run -it -v $(pwd):/ws ...
#COPY . .
