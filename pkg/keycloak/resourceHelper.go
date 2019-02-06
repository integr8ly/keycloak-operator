package keycloak

import (
	"github.com/ghodss/yaml"
	"github.com/integr8ly/keycloak-operator/pkg/apis/aerogear/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type ResourceHelper struct {
	templateHelper *MonitoringTemplateHelper
	cr             *v1alpha1.Keycloak
}

func newResourceHelper(cr *v1alpha1.Keycloak) *ResourceHelper {
	return &ResourceHelper{
		templateHelper: newTemplateHelper(cr),
		cr:             cr,
	}
}

func (r *ResourceHelper) createResource(template string) (*unstructured.Unstructured, error) {
	tpl, err := r.templateHelper.loadTemplate(template)
	if err != nil {
		return nil, err
	}

	resource := unstructured.Unstructured{}
	err = yaml.Unmarshal(tpl, &resource)

	if err != nil {
		return nil, err
	}

	return &resource, nil
}
