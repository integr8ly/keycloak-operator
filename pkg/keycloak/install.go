package keycloak

import (
	"fmt"
	"path/filepath"

	"github.com/aerogear/keycloak-operator/pkg/apis/aerogear/v1alpha1"
	"github.com/aerogear/keycloak-operator/pkg/apis/openshift/template"
	"github.com/aerogear/keycloak-operator/pkg/util"
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
	templateFilePath, err := filepath.Abs(fmt.Sprintf("%v/%v", SSO_TEMPLATE_PATH, SSO_TEMPLATE_NAME))
	if err != nil {
		return nil, err
	}
	res, err := util.LoadKubernetesResourceFromFile(templateFilePath)
	if err != nil {
		return nil, err
	}

	templ := res.(*v1.Template)
	fmt.Print("keycloak namespace ", keycloak.Namespace)
	processor, err := template.NewTemplateProcessor(keycloak.Namespace)
	if err != nil {
		return nil, err
	}

	return processor.Process(templ, params)
}
