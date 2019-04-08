ORG=integreatly
NAMESPACE=rhsso
CONSUMER_NAMESPACES=${NAMESPACE}
PROJECT=keycloak-operator
REG=quay.io
SHELL=/bin/bash
TAG=v1.3.0
PKG=github.com/integr8ly/keycloak-operator
TEST_DIRS?=$(shell sh -c "find $(TOP_SRC_DIRS) -name \\*_test.go -exec dirname {} \\; | sort | uniq")
TEST_POD_NAME=keycloak-operator-test
COMPILE_TARGET=./tmp/_output/bin/$(PROJECT)

.PHONY: setup/dep
setup/dep:
	@echo Installing dep
	curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh
	@echo setup complete

.PHONY: setup/travis
setup/travis:
	@echo Installing Operator SDK
	@curl -Lo operator-sdk https://github.com/operator-framework/operator-sdk/releases/download/v0.0.7/operator-sdk-v0.0.7-x86_64-linux-gnu && chmod +x operator-sdk && sudo mv operator-sdk /usr/local/bin/

.PHONY: code/run
code/run:
	export CONSUMER_NAMESPACES=${CONSUMER_NAMESPACES}
	@operator-sdk up local --namespace=${NAMESPACE} --operator-flags="--resync=10"

.PHONY: code/compile
code/compile:
	@GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o=$(COMPILE_TARGET) ./cmd/keycloak-operator

.PHONY: code/gen
code/gen:
	operator-sdk generate k8s
	@go generate ./...s

.PHONY: code/check
code/check:
	@diff -u <(echo -n) <(gofmt -d `find . -type f -name '*.go' -not -path "./vendor/*"`)

.PHONY: code/fix
code/fix:
	@gofmt -w `find . -type f -name '*.go' -not -path "./vendor/*"`

.PHONY: image/build
image/build: code/compile
	@operator-sdk build ${REG}/${ORG}/${PROJECT}:${TAG}

.PHONY: image/push
image/push:
	docker push ${REG}/${ORG}/${PROJECT}:${TAG}

.PHONY: image/build/push
image/build/push: image/build image/push

.PHONY: image/build/test
image/build/test:
	operator-sdk build --enable-tests docker.io/${ORG}/${PROJECT}:${TAG}

.PHONY: test/unit
test/unit:
	@echo Running tests:
	go test -v -race -cover ./pkg/...

.PHONY: test/e2e
test/e2e:
	kubectl apply -f deploy/test-e2e-pod.yaml -n ${PROJECT}
	${SHELL} ./scripts/stream-pod ${TEST_POD_NAME} ${PROJECT}

.PHONY: cluster/prepare
cluster/prepare:
	-kubectl apply -f deploy/crds/
	-oc new-project $(NAMESPACE)
	-kubectl create --insecure-skip-tls-verify -f deploy/rbac.yaml -n $(NAMESPACE)

.PHONY: cluster/clean
cluster/clean:
	-kubectl delete role keycloak-operator -n $(NAMESPACE)
	-kubectl delete rolebinding default-account-keycloak-operator -n $(NAMESPACE)
	-kubectl delete crd keycloaks.aerogear.org
	-kubectl delete namespace $(NAMESPACE)

.PHONY: cluster/create/examples
cluster/create/examples:
		-kubectl create -f deploy/examples/keycloak.json -n $(NAMESPACE)
		-kubectl create -f deploy/examples/keycloakRealm.json -n $(NAMESPACE)
