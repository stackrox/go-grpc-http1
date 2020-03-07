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

TESTFLAGS := -p 4 -race

### STYLE

.PHONY: style
style: imports lint vet

.PHONY: imports
imports: dev deps
	@echo "+ $@"
	@git ls-files -- '*.go' | xargs goimports -l -w

.PHONY: lint
lint: dev deps
	@echo "+ $@"
	@echo $(sort $(dir $(shell git ls-files -- '*.go'))) | xargs -n 1 golint

.PHONY: vet
vet: dev deps
	@echo "+ $@"
	@go vet ./...

.PHONY: dev
dev:
	@echo "+ $@"
	@go install golang.org/x/tools/cmd/goimports
	@go install golang.org/x/lint/golint
	@go install github.com/mauricelam/genny

deps: go.mod go.sum
	@echo "+ $@"
	@go mod tidy
	@$(MAKE) download-deps
	@touch deps

.PHONY: download-deps
download-deps:
	@echo "+ $@"
	@go mod download

### SOURCE GENERATION

.PHONY: generate-go-srcs
generate-go-srcs: dev
	@echo "+ $@"
	@go generate ./...

### UNIT TESTS

.PHONY: unit-tests
unit-tests: deps
	@echo "+ $@"
	@go test $(TESTFLAGS) ./...
