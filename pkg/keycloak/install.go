package keycloak

import (
	"github.com/integr8ly/keycloak-operator/pkg/apis/aerogear/v1alpha1"
	"github.com/integr8ly/keycloak-operator/pkg/apis/openshift/template"
	"github.com/integr8ly/keycloak-operator/pkg/util"
	"github.com/openshift/api/template/v1"
	"io/ioutil"
	"k8s.io/apimachinery/pkg/runtime"
)

func GetInstallResourcesAsRuntimeObjects(keycloak *v1alpha1.Keycloak, params map[string]string) ([]runtime.Object, error) {
	rawExtensions, err := GetInstallResources(keycloak, params)
	if err != nil {
		return nil, err
	}

	objects := make([]runtime.Object, 0)

	for _, rawObj := range rawExtensions {
		res, err := util.LoadKubernetesResource(rawObj.Raw)
		if err != nil {
			return nil, err
		}
		objects = append(objects, res)
	}

	return objects, nil
}

func GetInstallResources(keycloak *v1alpha1.Keycloak, params map[string]string) ([]runtime.RawExtension, error) {
	templateFilePath, err := getTemplatePath(SSO_TEMPLATE_NAME)
	if err != nil {
		return nil, err
	}

	tpl, err := ioutil.ReadFile(templateFilePath)
	if err != nil {
		return nil, err
	}

	jsonInjector := newJsonInjector()
	tpl, err = jsonInjector.InjectAll(tpl)
	if err != nil {
		return nil, err
	}

	res, err := util.LoadKubernetesResource(tpl)
	if err != nil {
		return nil, err
	}

	templ := res.(*v1.Template)
	processor, err := template.NewTemplateProcessor(keycloak.Namespace)
	if err != nil {
		return nil, err
	}

	return processor.Process(templ, params)
}
