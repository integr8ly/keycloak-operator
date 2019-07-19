package keycloak

import (
	"fmt"
	"github.com/Jeffail/gabs"
	"github.com/pkg/errors"
)

// JsonInjector provides functions to inject kubernetes resources into a JSON
// template
type JsonInjector interface {
	ParseTemplate(template []byte) (*gabs.Container, error)
	InjectEnvVar(*gabs.Container) (*gabs.Container, error)
	InjectVolumes(template []byte) ([]byte, error)
	InjectVolumeMounts(template []byte) ([]byte, error)
	InjectPostStartCommand(template []byte) ([]byte, error)
	InjectInitContainer(template []byte) ([]byte, error)
	InjectAll(template []byte) ([]byte, error)
}

type VolumeInfo struct {
	VolumeName      string
	VolumeMount     string
	InitVolumeMount string
}

// JsonInjectorImpl contains variables to be replaced by the injector
type JsonInjectorImpl struct {
	PluginsEnvVarName  string
	PluginsVolumeInfo  VolumeInfo
	ThemesVolumeInfo   VolumeInfo
	InitContainerName  string
	InitContainerMount string
	InitContainerImage string
}

// Create a new JsonInjector
func newJsonInjector() *JsonInjectorImpl {
	return &JsonInjectorImpl{
		PluginsEnvVarName:  "SSO_PLUGINS",
		PluginsVolumeInfo:  VolumeInfo{VolumeName: "sso-plugins", VolumeMount: "/opt/eap/providers", InitVolumeMount: "/opt/plugins"},
		ThemesVolumeInfo:   VolumeInfo{VolumeName: "sso-themes", VolumeMount: "/opt/themes", InitVolumeMount: "/opt/themes"},
		InitContainerName:  "sso-plugins-init",
		InitContainerImage: "quay.io/integreatly/sso_plugins_init:1.0.0",
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

	tpl, err = j.InjectVolumes(tpl)
	if err != nil {
		return nil, err
	}

	tpl, err = j.InjectVolumeMounts(tpl)
	if err != nil {
		return nil, err
	}

	tpl, err = j.InjectInitContainer(tpl)
	if err != nil {
		return nil, err
	}

	tpl, err = j.InjectPostStartCommand(tpl)
	if err != nil {
		return nil, err
	}

	tpl, err = j.NamePort(tpl)
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

// Find the DeploymentConfig resource that contains the sso containers
func (j *JsonInjectorImpl) LookupDeploymentConfig(tpl *gabs.Container) (*gabs.Container, error) {
	objects, err := tpl.S("objects").Children()

	if err != nil {
		return nil, err
	}

	for _, resource := range objects {
		kind := resource.S("kind").Data().(string)
		name := resource.S("metadata").S("name").Data().(string)

		// At this point the variables are not yet replaced. Our only indication that this is the
		// right deployment config is that the name will become `${APPLICATION_NAME}`
		if kind == "DeploymentConfig" && name == "${APPLICATION_NAME}" {
			return resource, nil
		}
	}

	return nil, errors.New("SSO DeploymentConfig not found")
}

// Find the SSO container in the DeploymentConfig
func (j *JsonInjectorImpl) LookupContainer(tpl *gabs.Container) (*gabs.Container, error) {
	containers, err := tpl.S("containers").Children()

	if err != nil {
		return nil, err
	}

	for _, container := range containers {
		name := container.S("name").Data().(string)

		// At this point the variables are not yet replaced. Our only indication that this is the
		// right container is that the name will become `${APPLICATION_NAME}`
		if name == "${APPLICATION_NAME}" {
			return container, nil
		}
	}

	return nil, errors.New("SSO Container not found")
}

// Find the service for the SSO pod
func (j *JsonInjectorImpl) LookupSSOService(tpl *gabs.Container) (*gabs.Container, error) {
	objects, err := tpl.S("objects").Children()

	if err != nil {
		return nil, err
	}

	for _, resource := range objects {
		kind := resource.S("kind").Data().(string)
		name := resource.S("metadata").S("name").Data().(string)

		// At this point the variables are not yet replaced. Our only indication that this is the
		// right deployment config is that the name will become `${APPLICATION_NAME}`
		if kind == "Service" && name == "${APPLICATION_NAME}" {
			return resource, nil
		}
	}

	return nil, errors.New("SSO DeploymentConfig not found")
}

// In order for the metrics endpoint to be disoverable, the port of the SSO service
// has to be named. This function adds a name field to the port.
func (j *JsonInjectorImpl) NamePort(tpl *gabs.Container) (*gabs.Container, error) {
	service, err := j.LookupSSOService(tpl)
	if err != nil {
		return nil, err
	}

	_, err = service.S("spec").S("ports").Index(0).Set("sso", "name")
	if err != nil {
		return nil, err
	}

	return tpl, nil
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
func (j *JsonInjectorImpl) InjectVolumes(tpl *gabs.Container) (*gabs.Container, error) {
	// Path in the template is .objects[4].spec.template.spec.volumes
	deploymentConfig, err := j.LookupDeploymentConfig(tpl)
	if err != nil {
		return nil, err
	}

	for _, volume := range []VolumeInfo{j.PluginsVolumeInfo, j.ThemesVolumeInfo} {
		pluginsVolume := gabs.New()
		pluginsVolume.Set(volume.VolumeName, "name")
		pluginsVolume.Set(gabs.New().Data(), "emptyDir")

		err = deploymentConfig.S("spec").S("template").S("spec").ArrayAppend(pluginsVolume.Data(), "volumes")
		if err != nil {
			return nil, err
		}
	}

	return tpl, nil
}

// Injects a volume mount into the RHSSO container that points to the installed
// plugins. It has to be mounted at the path where keycloak loads plugins at startup
func (j *JsonInjectorImpl) InjectVolumeMounts(tpl *gabs.Container) (*gabs.Container, error) {
	// Path in the template is .objects[4].spec.template.spec.containers[0].volumeMounts
	deploymentConfig, err := j.LookupDeploymentConfig(tpl)
	if err != nil {
		return nil, err
	}

	container, err := j.LookupContainer(deploymentConfig.S("spec").S("template").S("spec"))
	if err != nil {
		return nil, err
	}

	for _, volume := range []VolumeInfo{j.PluginsVolumeInfo, j.ThemesVolumeInfo} {
		volumeMount := gabs.New()
		volumeMount.Set(volume.VolumeMount, "mountPath")
		volumeMount.Set(volume.VolumeName, "name")
		volumeMount.Set(false, "readonly")

		container.ArrayAppend(volumeMount.Data(), "volumeMounts")
		if err != nil {
			return nil, err
		}
	}

	return tpl, nil
}

// Injects a lifecycle.postStart.exec.command that will copy theme files into SSO container
func (j *JsonInjectorImpl) InjectPostStartCommand(tpl *gabs.Container) (*gabs.Container, error) {
	// Path in the template is .objects[4].spec.template.spec.containers[0].volumeMounts
	deploymentConfig, err := j.LookupDeploymentConfig(tpl)
	if err != nil {
		return nil, err
	}

	container, err := j.LookupContainer(deploymentConfig.S("spec").S("template").S("spec"))
	if err != nil {
		return nil, err
	}

	command := []string{"/bin/sh", "-c", "cp -RT /opt/themes /opt/eap/themes"}
	container.Set(command, "lifecycle", "postStart", "exec", "command")

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
	initContainer.ArrayAppend(initContainerEnv.Data(), "env")

	// Init container volume mounts
	for _, volume := range []VolumeInfo{j.PluginsVolumeInfo, j.ThemesVolumeInfo} {
		initContainerMount := gabs.New()
		initContainerMount.Set(volume.InitVolumeMount, "mountPath")
		initContainerMount.Set(volume.VolumeName, "name")
		initContainerMount.Set(false, "readonly")

		initContainer.ArrayAppend(initContainerMount.Data(), "volumeMounts")
	}

	// Init container name and image
	initContainer.Set(j.InitContainerImage, "image")
	initContainer.Set(j.InitContainerName, "name")

	// Path in the template is .objects[4].spec.template.spec.initContainers
	deploymentConfig, err := j.LookupDeploymentConfig(tpl)
	if err != nil {
		return nil, err
	}

	err = deploymentConfig.S("spec").S("template").S("spec").ArrayAppend(initContainer.Data(), "initContainers")
	if err != nil {
		return nil, err
	}

	return tpl, nil
}
