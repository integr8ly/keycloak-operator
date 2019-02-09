package keycloak

import (
	"bytes"
	"fmt"
	"github.com/integr8ly/keycloak-operator/pkg/apis/aerogear/v1alpha1"
	"io/ioutil"
	"os"
	"path/filepath"
	"text/template"
)

const (
	GrafanaDashboardName = "grafana-dashboard"
	PrometheusRuleName   = "prometheus-rule"
	ServiceMonitorName   = "service-monitor"
)

type MonitoringParameters struct {
	MonitoringKey string
	Namespace     string
}

type MonitoringTemplateHelper struct {
	Parameters   MonitoringParameters
	TemplatePath string
}

// Return either the default template path (deploy/template) or an overridden value
// from an env var
func getTemplatePath(template string) (string, error) {
	var templateFilePath string
	var err error

	templatePathEnvVar, found := os.LookupEnv(SSO_TEMPLATE_PATH_ENV_VAR)
	if found {
		templateFilePath, err = filepath.Abs(fmt.Sprintf("%v/%v", templatePathEnvVar, template))
	} else {
		templateFilePath, err = filepath.Abs(fmt.Sprintf("%v/%v", SSO_TEMPLATE_PATH, template))
	}

	if err != nil {
		return "", err
	}

	return templateFilePath, nil
}

// Creates a new template helper and populates the values for all
// templates properties
func newTemplateHelper(sso *v1alpha1.Keycloak) *MonitoringTemplateHelper {
	param := MonitoringParameters{
		Namespace:     sso.Namespace,
		MonitoringKey: "middleware",
	}

	monitoringKey, exists := os.LookupEnv("MONITORING_KEY")
	if exists {
		param.MonitoringKey = monitoringKey
	}

	return &MonitoringTemplateHelper{
		Parameters: param,
	}
}

// load a templates from a given resource name
func (h *MonitoringTemplateHelper) loadTemplate(name string) ([]byte, error) {
	templatePath, err := getTemplatePath(name)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("%s.yaml", templatePath)
	tpl, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Use [[ ]] as delimiters because {{ }} is extensively used in the
	// prometheus rules and grafana dashboard
	parsed, err := template.New("monitoring").Delims("[[", "]]").Parse(string(tpl))
	if err != nil {
		return nil, err
	}

	var buffer bytes.Buffer
	err = parsed.Execute(&buffer, h.Parameters)
	if err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}
