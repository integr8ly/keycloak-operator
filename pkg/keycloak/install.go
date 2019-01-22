package keycloak

import (
	"fmt"
	"github.com/Jeffail/gabs"
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

func ModifyTemplate(template []byte) ([]byte, error) {
	parsed, err := gabs.ParseJSON(template)
	if err != nil {
		return nil, err
	}

	ssoPlugins := gabs.New()
	ssoPlugins.Set("SSO_PLUGINS", "name")
	ssoPlugins.Set("Keycloak Plugins", "description")
	ssoPlugins.Set("RH-SSO Installed Plugins", "displayName")
	parsed.ArrayAppend(ssoPlugins.Data(), "parameters")

	pluginsVolume := gabs.New()
	pluginsVolume.Set("sso-plugins", "name")
	pluginsVolume.Set(gabs.New().Data(), "emptyDir")
	parsed.S("objects").Index(4).S("spec").S("template").S("spec").ArrayAppend(pluginsVolume.Data(), "volumes")

	volumeMount := gabs.New()
	volumeMount.Set("/opt/eap/providers", "mountPath")
	volumeMount.Set("sso-plugins", "name")
	volumeMount.Set(false, "readonly")
	parsed.S("objects").Index(4).S("spec").S("template").S("spec").S("containers").Index(0).ArrayAppend(volumeMount.Data(), "volumeMounts")

	initContainer := gabs.New()
	initContainerEnv := gabs.New()
	initContainerEnv.Set("SSO_PLUGINS", "name")
	initContainerEnv.Set("${SSO_PLUGINS}", "value")

	initContainerMount := gabs.New()
	initContainerMount.Set("/opt/plugins", "mountPath")
	initContainerMount.Set("sso-plugins", "name")
	initContainerMount.Set(false, "readonly")

	initContainer.ArrayAppend(initContainerEnv.Data(), "env")
	initContainer.ArrayAppend(initContainerMount.Data(), "volumeMounts")

	initContainer.Set("docker.io/pb82/kc_plugins_init:latest", "image")
	initContainer.Set("sso-plugins-init", "name")
	parsed.S("objects").Index(4).S("spec").S("template").S("spec").ArrayAppend(initContainer.Data(), "initContainers")

	return parsed.Bytes(), nil
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

	tpl, err = ModifyTemplate(tpl)
	if err != nil {
		return nil, err
	}

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
