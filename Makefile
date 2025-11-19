# Copyright 2025 DoorDash, Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

GOBASE=$(shell pwd)
GOBIN=$(GOBASE)/bin

help:
	@echo "This is a helper makefile for oapi-codegen"
	@echo "Targets:"
	@echo "    generate:    regenerate all generated files"
	@echo "    test:        run all tests"
	@echo "    tidy         tidy go mod"
	@echo "    lint         lint the project"

$(GOBIN)/golangci-lint:
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(GOBIN) v2.4.0

.PHONY: tools
tools: $(GOBIN)/golangci-lint

lint: tools
	# run the root module explicitly, to prevent recursive calls by re-invoking `make ...` top-level
	$(GOBIN)/golangci-lint run ./...
	# then, for all child modules, use a module-managed `Makefile`
	git ls-files '**/*go.mod' -z | xargs -0 -I{} bash -xc 'cd $$(dirname {}) && env GOBIN=$(GOBIN) make lint'

lint-ci: tools
	# for the root module, explicitly run the step, to prevent recursive calls
	$(GOBIN)/golangci-lint run ./... --out-format=colored-line-number --timeout=5m
	# then, for all child modules, use a module-managed `Makefile`
	git ls-files '**/*go.mod' -z | xargs -0 -I{} bash -xc 'cd $$(dirname {}) && env GOBIN=$(GOBIN) make lint-ci'

generate:
	# for the root module, explicitly run the step, to prevent recursive calls
	go generate ./...
	# then, for all child modules, use a module-managed `Makefile`
	git ls-files '**/*go.mod' -z | xargs -0 -I{} bash -xc 'cd $$(dirname {}) && make generate'

test:
	# for the root module, explicitly run the step, to prevent recursive calls
	go test -cover ./...
	# then, for all child modules, use a module-managed `Makefile`
	git ls-files '**/*go.mod' -z | xargs -0 -I{} bash -xc 'cd $$(dirname {}) && make test'

tidy:
	# for the root module, explicitly run the step, to prevent recursive calls
	go mod tidy
	# then, for all child modules, use a module-managed `Makefile`
	git ls-files '**/*go.mod' -z | xargs -0 -I{} bash -xc 'cd $$(dirname {}) && make tidy'

tidy-ci:
	# for the root module, explicitly run the step, to prevent recursive calls
	tidied -verbose
	# then, for all child modules, use a module-managed `Makefile`
	git ls-files '**/*go.mod' -z | xargs -0 -I{} bash -xc 'cd $$(dirname {}) && make tidy-ci'

test-integration:
	go test -v -tags=integration ./pkg/codegen/integration/...

check-all: generate lint test test-integration
	@if [ -n "$$(git status --porcelain)" ]; then \
		echo >&2 "ERROR: generate command should not produce extra code"; \
		exit 1; \
	fi
