#   Copyright The containerd Authors.

#   Licensed under the Apache License, Version 2.0 (the "License");
#   you may not use this file except in compliance with the License.
#   You may obtain a copy of the License at

#       http://www.apache.org/licenses/LICENSE-2.0

#   Unless required by applicable law or agreed to in writing, software
#   distributed under the License is distributed on an "AS IS" BASIS,
#   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#   See the License for the specific language governing permissions and
#   limitations under the License.

ARG GO_VERSION=1.24

FROM golang:${GO_VERSION}-bullseye AS builder
ARG PLUGIN

WORKDIR /go/src

# Cache dependencies in a separate layer
COPY go.mod go.sum .
RUN --mount=type=cache,target=/go/pkg/mod/ go mod download

# Build
COPY . .

RUN --mount=type=cache,target=/go/pkg/mod/ \
    make build/bin/${PLUGIN} \
    GO_BUILD="CGO_ENABLED=0 go build" \
    GO_BUILD_FLAGS="-ldflags '-extldflags=-static'"

# Construct final image
FROM scratch
ARG PLUGIN

COPY --from=builder /go/src/build/bin/${PLUGIN} /bin/plugin

ENV NRI_PLUGIN_NAME=${PLUGIN}

ENTRYPOINT ["/bin/plugin"]
