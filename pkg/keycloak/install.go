package keycloak

import (
	"fmt"
	"path/filepath"

	"github.com/aerogear/keycloak-operator/pkg/apis/aerogear/v1alpha1"
	"github.com/aerogear/keycloak-operator/pkg/apis/openshift/template"
	"github.com/aerogear/keycloak-operator/pkg/util"
	"github.com/openshift/api/template/v1"
	"github.com/operator-framework/operator-sdk/pkg/k8sclient"
	"github.com/operator-framework/operator-sdk/pkg/util/k8sutil"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
)

func install(kc *v1alpha1.Keycloak, params map[string]string) error {
	objects, err := GetInstallResourcesAsRuntimeObjects(kc, params)
	if err != nil {
		return err
	}
	namespace := kc.Namespace
	for _, o := range objects {
		gvk := o.GetObjectKind().GroupVersionKind()

		apiVersion, kind := gvk.ToAPIVersionAndKind()
		resourceClient, _, err := k8sclient.GetResourceClient(apiVersion, kind, namespace)
		if err != nil {
			return fmt.Errorf("failed to get resource client: %v", err)
		}

		unstructObj, err := k8sutil.UnstructuredFromRuntimeObject(o)
		if err != nil {
			return err
		}
		unstructObj, err = resourceClient.Create(unstructObj)
		if err != nil && !errors.IsAlreadyExists(err) {
			return err
		}

	}
	return nil
}

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
