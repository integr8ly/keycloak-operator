#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

if ! which go > /dev/null; then
	echo "golang needs to be installed"
	exit 1
fi

BIN_DIR="$(pwd)/tmp/_output/bin"
TEMPLATE_DIR="$(pwd)/tmp/_output/deploy/template"
mkdir -p ${BIN_DIR}
mkdir -p ${TEMPLATE_DIR}
cp $(pwd)/deploy/template/* ${TEMPLATE_DIR}

PROJECT_NAME="keycloak-operator"
REPO_PATH="github.com/aerogear/keycloak-operator"
BUILD_PATH="${REPO_PATH}/cmd/${PROJECT_NAME}"
echo "building "${PROJECT_NAME}"..."
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o ${BIN_DIR}/${PROJECT_NAME} $BUILD_PATH
