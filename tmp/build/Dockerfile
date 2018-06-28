FROM alpine:3.6

RUN adduser -D keycloak-operator
USER keycloak-operator

ADD tmp/_output/bin/keycloak-operator /usr/local/bin/keycloak-operator
