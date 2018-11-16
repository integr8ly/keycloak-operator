ARG BASEIMAGE

FROM ${BASEIMAGE}

ADD tmp/_output/bin/keycloak-operator-test /usr/local/bin/keycloak-operator-test
ARG NAMESPACEDMAN
ADD $NAMESPACEDMAN /namespaced.yaml
ADD tmp/build/go-test.sh /go-test.sh
