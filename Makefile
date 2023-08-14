# Copyright (c) 2020 StackRox Inc.
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

export GO111MODULE := on
export GOBIN := $(CURDIR)/.gobin
export PATH := $(GOBIN):$(PATH)

# Set to empty string to echo some command lines which are hidden by default.
SILENT ?= @

TESTFLAGS := -p 4 -race

### STYLE

.PHONY: style
style: imports lint vet

.PHONY: imports
imports: dev deps
	$(SILENT)echo "+ $@"
	$(SILENT)git ls-files -- '*.go' | xargs goimports -l -w

.PHONY: lint
lint: dev deps
	$(SILENT)echo "+ $@"
	$(SILENT)echo $(sort $(dir $(shell git ls-files -- '*.go'))) | xargs -n 1 golint

.PHONY: vet
vet: dev deps
	$(SILENT)echo "+ $@"
	$(SILENT)go vet ./...

.PHONY: dev
dev:
	$(SILENT)echo "+ $@"
	$(SILENT)cd tools/ && go install golang.org/x/tools/cmd/goimports
	$(SILENT)cd tools/ && go install golang.org/x/lint/golint

deps: go.mod go.sum
	$(SILENT)echo "+ $@"
	$(SILENT)go mod tidy
	$(SILENT)$(MAKE) download-deps
	$(SILENT)touch deps

.PHONY: integration-deps
integration-deps:
	$(SILENT)echo "+ $@"
	$(SILENT)cd "_integration-tests";\
		go mod tidy;\
		go mod download

.PHONY: download-deps
download-deps:
	$(SILENT)echo "+ $@"
	$(SILENT)go mod download

### SOURCE GENERATION

.PHONY: generate-go-srcs
generate-go-srcs: dev
	$(SILENT)echo "+ $@"
	$(SILENT)go generate ./...

### TESTS

.PHONY: unit-tests
unit-tests: deps
	$(SILENT)echo "+ $@"
	$(SILENT)go test $(TESTFLAGS) ./...

.PHONY: integration-tests
integration-tests: integration-deps
	$(SILENT)echo "+ $@"
	$(SILENT)cd _integration-tests ; go test -count=1 -p 4 .
