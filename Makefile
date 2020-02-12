# Copyright 2019 The Caicloud Authors.
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

BIN_DIR=_output/bin
REPO_PATH=github.com/angao/coscheduling

# If tag not explicitly set in users default to the git sha.
TAG ?= ${shell git describe --tags `git rev-list --tags --max-count=1` 2>/dev/null || git rev-parse --short HEAD}
GitSHA=${shell git rev-parse --verify --short HEAD}
Date=${shell date "+%Y-%m-%d %H:%M:%S"}
LD_FLAGS="\
    -X '${REPO_PATH}/pkg/version.GitSHA=${GitSHA}' \
    -X '${REPO_PATH}/pkg/version.BuiltDate=${Date}'   \
    -X '${REPO_PATH}/pkg/version.Version=${TAG}'"

.EXPORT_ALL_VARIABLES:

all: build-local

init:
	mkdir -p ${BIN_DIR}

fmt:
	go fmt ./pkg/...

vet:
	go vet ./pkg/...

build-local: init fmt
	go build -mod=vendor -ldflags ${LD_FLAGS} -o=${BIN_DIR}/coscheduling ./cmd/scheduler

build-linux: init fmt
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod=vendor -ldflags ${LD_FLAGS} -o=${BIN_DIR}/coscheduling ./cmd/scheduler

image: build-linux
	docker build --no-cache -f ./build/Dockerfile . -t coscheduling:$(TAG)

gen-code:
	./hack/update-codegen.sh

verify-code:
	./hack/verify-codegen.sh

unit-test:
	go list ./... | grep -v e2e | xargs go test -v -race

clean:
	rm -rf _output/
	rm -f *.log
