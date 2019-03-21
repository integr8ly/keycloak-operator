FROM alpine:3.6

RUN adduser -D keycloak-operator
USER keycloak-operator
ENV PATH="/home/keycloak-operator:${PATH}"
ENV TEMPLATE_DIR="/home/keycloak-operator/deploy/template"
ADD tmp/_output/bin/keycloak-operator /home/keycloak-operator/keycloak-operator
ADD deploy/template/sso73-x509-postgresql-persistent.json /home/keycloak-operator/deploy/template/sso73-x509-postgresql-persistent.json
ADD deploy/template/sso72-x509-postgresql-persistent.json /home/keycloak-operator/deploy/template/sso72-x509-postgresql-persistent.json
ADD deploy/template/prometheus-rule.yaml /home/keycloak-operator/deploy/template/prometheus-rule.yaml
ADD deploy/template/service-monitor.yaml /home/keycloak-operator/deploy/template/service-monitor.yaml
ADD deploy/template/grafana-dashboard.yaml /home/keycloak-operator/deploy/template/grafana-dashboard.yaml


