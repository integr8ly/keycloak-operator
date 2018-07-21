ORG=aerogear
NAMESPACE=shared
PROJECT=keycloak-operator
SHELL = /bin/bash
TAG = 0.0.2
PKG = github.com/aerogear/keycloak-operator
TEST_DIRS     ?= $(shell sh -c "find $(TOP_SRC_DIRS) -name \\*_test.go -exec dirname {} \\; | sort | uniq")

.PHONY: check-gofmt
check-gofmt:
	diff -u <(echo -n) <(gofmt -d `find . -type f -name '*.go' -not -path "./vendor/*"`)



.PHONY: test-unit
test-unit:
	@echo Running tests:
	go test -v -race -cover ./pkg/...

.PHONY: setup
setup:
	@echo Installing operator-sdk cli
	cd vendor/github.com/operator-framework/operator-sdk/commands/operator-sdk/ && go install .
	@echo Installing dep
	curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh
	@echo Installing errcheck
	@go get github.com/kisielk/errcheck
	@echo setup complete run make build deploy to build and deploy the operator to a local cluster


.PHONY: build
build-image:
	operator-sdk build quay.io/${ORG}/${PROJECT}:${TAG}

.PHONY: run
run:
	operator-sdk up local --namespace=${NAMESPACE}

compile:
	go build -o=keycloak-operator ./cmd/keycloak-operator

.PHONY: check
check: check-gofmt test-unit
	@echo errcheck
	@errcheck -ignoretests $$(go list ./...)
	@echo go vet
	@go vet ./...