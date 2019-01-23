package keycloak

import (
	"fmt"
	"github.com/Jeffail/gabs"
)

// JsonInjector provides functions to inject kubernetes resources into a JSON
// template
type JsonInjector interface {
	ParseTemplate(template []byte) (*gabs.Container, error)
	InjectEnvVar(*gabs.Container) (*gabs.Container, error)
	InjectVolume(template []byte) ([]byte, error)
	InjectVolumeMount(template []byte) ([]byte, error)
	InjectInitContainer(template []byte) ([]byte, error)
	InjectAll(template []byte) ([]byte, error)
}

// JsonInjectorImpl contains variables to be replaced by the injector
type JsonInjectorImpl struct {
	PluginsEnvVarName  string
	PluginsVolumeName  string
	PluginVolumeMount  string
	InitContainerName  string
	InitContainerMount string
	InitContainerImage string
}

// Create a new JsonInjector
func newJsonInjector() *JsonInjectorImpl {
	return &JsonInjectorImpl{
		PluginsEnvVarName:  "SSO_PLUGINS",
		PluginsVolumeName:  "sso-plugins",
		PluginVolumeMount:  "/opt/eap/providers",
		InitContainerName:  "sso-plugins-init",
		InitContainerMount: "/opt/plugins",
		InitContainerImage: "docker.io/pb82/kc_plugins_init:latest",
	}
}

// Inject all Kubernetes resources to bring up the plugins init container
// This includes the env var, volume, mount and init container
func (j *JsonInjectorImpl) InjectAll(template []byte) ([]byte, error) {
	tpl, err := j.ParseTemplate(template)
	if err != nil {
		return nil, err
	}

	tpl, err = j.InjectEnvVar(tpl)
	if err != nil {
		return nil, err
	}

	tpl, err = j.InjectVolume(tpl)
	if err != nil {
		return nil, err
	}

	tpl, err = j.InjectVolumeMount(tpl)
	if err != nil {
		return nil, err
	}

	tpl, err = j.InjectInitContainer(tpl)
	if err != nil {
		return nil, err
	}

	return tpl.Bytes(), nil
}

// Parse the string template to a mutable JSON object
func (j *JsonInjectorImpl) ParseTemplate(template []byte) (*gabs.Container, error) {
	parsed, err := gabs.ParseJSON(template)
	if err != nil {
		return nil, err
	}
	return parsed, nil
}

// Injects an env var into the Pod containing the list of plugins
// The actual list is provided by the operator when the deployment is created
func (j *JsonInjectorImpl) InjectEnvVar(tpl *gabs.Container) (*gabs.Container, error) {
	ssoPlugins := gabs.New()
	ssoPlugins.Set(j.PluginsEnvVarName, "name")
	ssoPlugins.Set("RH-SSO Installed Plugins", "description")
	ssoPlugins.Set("RH-SSO Installed Plugins", "displayName")

	// Path in the template is .parameters
	err := tpl.ArrayAppend(ssoPlugins.Data(), "parameters")

	if err != nil {
		return nil, err
	}

	return tpl, nil
}

// Injects a volume of type emptyDir into the RHSSO pod that is used to store the
// installed plugins
func (j *JsonInjectorImpl) InjectVolume(tpl *gabs.Container) (*gabs.Container, error) {
	pluginsVolume := gabs.New()
	pluginsVolume.Set(j.PluginsVolumeName, "name")
	pluginsVolume.Set(gabs.New().Data(), "emptyDir")

	// Path in the template is .objects[4].spec.template.spec.volumes
	err := tpl.S("objects").Index(4).S("spec").S("template").S("spec").ArrayAppend(pluginsVolume.Data(), "volumes")

	if err != nil {
		return nil, err
	}

	return tpl, nil
}

// Injects a volume mount into the RHSSO container that points to the installed
// plugins. It has to be mounted at the path where keycloak loads plugins at startup
func (j *JsonInjectorImpl) InjectVolumeMount(tpl *gabs.Container) (*gabs.Container, error) {
	volumeMount := gabs.New()
	volumeMount.Set(j.PluginVolumeMount, "mountPath")
	volumeMount.Set(j.PluginsVolumeName, "name")
	volumeMount.Set(false, "readonly")

	// Path in the template is .objects[4].spec.template.spec.containers[0].volumeMounts
	err := tpl.S("objects").Index(4).S("spec").S("template").S("spec").S("containers").Index(0).ArrayAppend(volumeMount.Data(), "volumeMounts")

	if err != nil {
		return nil, err
	}

	return tpl, nil
}

// Injects the init container into the RHSSO pod that will copy all plugin binaries from
// the init container into the mounted volume
func (j *JsonInjectorImpl) InjectInitContainer(tpl *gabs.Container) (*gabs.Container, error) {
	// Init container base object
	initContainer := gabs.New()

	// Init container env section
	initContainerEnv := gabs.New()
	initContainerEnv.Set(j.PluginsEnvVarName, "name")
	initContainerEnv.Set(fmt.Sprintf("${%s}", j.PluginsEnvVarName), "value")

	// Init container volume mounts
	initContainerMount := gabs.New()
	initContainerMount.Set(j.InitContainerMount, "mountPath")
	initContainerMount.Set(j.PluginsVolumeName, "name")
	initContainerMount.Set(false, "readonly")

	initContainer.ArrayAppend(initContainerEnv.Data(), "env")
	initContainer.ArrayAppend(initContainerMount.Data(), "volumeMounts")

	// Init container name and image
	initContainer.Set(j.InitContainerImage, "image")
	initContainer.Set(j.InitContainerName, "name")

	// Path in the template is .objects[4].spec.template.spec.initContainers
	err := tpl.S("objects").Index(4).S("spec").S("template").S("spec").ArrayAppend(initContainer.Data(), "initContainers")

	if err != nil {
		return nil, err
	}

	return tpl, nil
}
