package keycloak

import (
	"fmt"
	"github.com/integr8ly/keycloak-operator/pkg/apis/openshift/template"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/integr8ly/keycloak-operator/pkg/apis/aerogear/v1alpha1"
	"github.com/integr8ly/keycloak-operator/pkg/util"
	"github.com/openshift/api/template/v1"
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
	var templateFilePath string
	var err error
	templatePathEnvVar, found := os.LookupEnv(SSO_TEMPLATE_PATH_ENV_VAR)

	if found {
		templateFilePath, err = filepath.Abs(fmt.Sprintf("%v/%v", templatePathEnvVar, SSO_TEMPLATE_NAME))
	} else {
		templateFilePath, err = filepath.Abs(fmt.Sprintf("%v/%v", SSO_TEMPLATE_PATH, SSO_TEMPLATE_NAME))
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
