package keycloak

import (
	v1 "github.com/openshift/api/apps/v1"
	"github.com/sirupsen/logrus"
	cv1 "k8s.io/api/core/v1"
	"regexp"
)

func CanUpgrade(version string) bool {
	// we will handle upgrade for any 7.2.x version
	r := regexp.MustCompile("^v7.2.*.GA$")
	return r.MatchString(version)
}

func DeploymentUpgraded(dc *v1.DeploymentConfig) bool {
	if !triggersUpgraded(dc) {
		logrus.Debug("triggers are not upgraded")
		return false
	}
	if !volumesUpgraded(dc) {
		logrus.Debug("volumes are not upgraded")
		return false
	}

	return envVarsAndVolumeMountsUpgraded(dc)
}

func triggersUpgraded(dc *v1.DeploymentConfig) bool {
	for _, t := range dc.Spec.Triggers {
		if t.Type == v1.DeploymentTriggerOnImageChange {
			if t.ImageChangeParams.From.Name == SSO_IMAGE_STREAM {
				return true
			}
		}
	}
	return false
}

func volumesUpgraded(dc *v1.DeploymentConfig) bool {
	for _, v := range dc.Spec.Template.Spec.Volumes {
		if v.Name == "sso-x509-jgroups-volume" {
			return true
		}
	}
	return false
}

func envVarsAndVolumeMountsUpgraded(dc *v1.DeploymentConfig) bool {
	jgroupEnvFound := false
	ssoHostEnvFound := false
	for _, c := range dc.Spec.Template.Spec.Containers {
		if c.Name == SSO_APPLICATION_NAME {
			volumeMountFound := false
			for _, vm := range c.VolumeMounts {
				if vm.Name == "sso-x509-jgroups-volume" {
					volumeMountFound = true
				}
			}
			if !volumeMountFound {
				logrus.Debug("volume mound missing")
				return false
			}
			for _, e := range c.Env {
				if e.Name == "SSO_HOSTNAME" {
					ssoHostEnvFound = true
				}
				if e.Name == "JGROUPS_ENCRYPT_PROTOCOL" {
					jgroupEnvFound = true
				}
			}
			break
		}
	}
	if jgroupEnvFound == true || !ssoHostEnvFound {
		logrus.Debug("env vars are not correct")
		return false
	}
	return true
}

func ServiceUpgraded(service *cv1.Service) bool {
	_, ok := service.ObjectMeta.Annotations["service.alpha.openshift.io/serving-cert-secret-name"]
	return ok
}

func UpgradeDeploymentConfig(dc *v1.DeploymentConfig) *v1.DeploymentConfig {
	for i, _ := range dc.Spec.Template.Spec.InitContainers {
		if dc.Spec.Template.Spec.InitContainers[i].Name == "sso-plugins-init" {
			logrus.Infof("updated init container image")
			dc.Spec.Template.Spec.InitContainers[i].Image = "quay.io/integreatly/sso_plugins_init:0.0.3"
		}
	}
	for _, t := range dc.Spec.Triggers {
		if t.Type == v1.DeploymentTriggerOnImageChange {
			t.ImageChangeParams.From.Name = SSO_IMAGE_STREAM
		}
	}
	volumeExists := false
	volumeName := "sso-x509-jgroups-volume"
	for _, v := range dc.Spec.Template.Spec.Volumes {
		if v.Name == volumeName {
			volumeExists = true
			break
		}
	}
	if !volumeExists {
		dc.Spec.Template.Spec.Volumes = append(dc.Spec.Template.Spec.Volumes, cv1.Volume{
			Name: volumeName,
			VolumeSource: cv1.VolumeSource{
				Secret: &cv1.SecretVolumeSource{
					SecretName: "sso-x509-jgroups-secret",
				},
			},
		})
	}
	for i, c := range dc.Spec.Template.Spec.Containers {
		if c.Name == SSO_APPLICATION_NAME {
			// check if the volume already exists
			volumeMountName := "sso-x509-jgroups-volume"
			volumeMountExists := false
			for _, vm := range dc.Spec.Template.Spec.Containers[i].VolumeMounts {
				if vm.Name == volumeMountName {
					volumeMountExists = true
					break
				}
			}
			if !volumeMountExists {
				dc.Spec.Template.Spec.Containers[i].VolumeMounts = append(c.VolumeMounts, cv1.VolumeMount{
					Name:      volumeMountName,
					MountPath: "/etc/x509/jgroups",
					ReadOnly:  true,
				})
			}
			var ei = 0
			var e = cv1.EnvVar{}
			for ei, e = range c.Env {
				if e.Name == "JGROUPS_ENCRYPT_PROTOCOL" {
					break
				}
			}
			dc.Spec.Template.Spec.Containers[i].Env = append(c.Env[:ei], c.Env[ei+1:]...)
			dc.Spec.Template.Spec.Containers[i].Env = append(dc.Spec.Template.Spec.Containers[i].Env, cv1.EnvVar{
				Name:  "SSO_HOSTNAME",
				Value: "",
			})
			return dc
		}
	}
	return dc
}

func UpgradeService(s *cv1.Service) *cv1.Service {
	s.ObjectMeta.Annotations["service.alpha.openshift.io/serving-cert-secret-name"] = "sso-x509-jgroups-secret"
	return s
}
