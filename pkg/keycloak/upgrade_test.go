package keycloak_test

import (
	"fmt"
	"github.com/integr8ly/keycloak-operator/pkg/keycloak"
	v1 "github.com/openshift/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func TestCanUpgrade(t *testing.T) {
	cases := []struct {
		Name     string
		Versions []string
		Expected bool
	}{
		{
			Name:     "Test Should upgrade",
			Versions: []string{"v7.3.11.GA", "v7.3.1.GA"},
			Expected: true,
		},
		{
			Name:     "Test Should Not upgrade",
			Versions: []string{"v7.4.2.GA", "v7.4.1.GA", "v7.4.0.GA", "v7.3.0-ALPHA"},
			Expected: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			for _, v := range tc.Versions {
				ex := keycloak.CanUpgrade(v)
				if ex != tc.Expected {
					t.Fatal("expected ", tc.Expected, " but got ", ex)
				}
			}
		})
	}
}

func TestUpgradeService(t *testing.T) {
	cases := []struct {
		Name     string
		SVC      *corev1.Service
		Validate func(t *testing.T, s *corev1.Service)
	}{
		{
			Name: "test service upgraded as expected",
			SVC: &corev1.Service{
				ObjectMeta: v12.ObjectMeta{Annotations: map[string]string{}},
			},
			Validate: func(t *testing.T, s *corev1.Service) {
				v, ok := s.Annotations["service.alpha.openshift.io/serving-cert-secret-name"]
				if !ok {
					t.Fatal("expected the annotation ", "service.alpha.openshift.io/serving-cert-secret-name", "to be present")
				}
				if v != "sso-x509-jgroups-secret" {
					t.Fatal("expected the annotation service.alpha.openshift.io/serving-cert-secret-name to be sso-x509-jgroups-secret but it was  ", v)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			tc.Validate(t, keycloak.UpgradeService(tc.SVC))
		})
	}
}

func TestDeploymentUpgraded(t *testing.T) {
	dc := &v1.DeploymentConfig{
		Spec: v1.DeploymentConfigSpec{
			Triggers: v1.DeploymentTriggerPolicies{
				v1.DeploymentTriggerPolicy{
					Type: v1.DeploymentTriggerOnImageChange,
					ImageChangeParams: &v1.DeploymentTriggerImageChangeParams{
						From: corev1.ObjectReference{
							Name: keycloak.SSO_IMAGE_STREAM,
						},
					},
				},
			},
			Template: &corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						corev1.Volume{Name: "sso-x509-jgroups-volume"},
						corev1.Volume{},
					},
					Containers: []corev1.Container{
						corev1.Container{
							Name: keycloak.SSO_APPLICATION_NAME,
							VolumeMounts: []corev1.VolumeMount{
								corev1.VolumeMount{Name: "sso-x509-jgroups-volume"},
								corev1.VolumeMount{},
							},
							Env: []corev1.EnvVar{
								corev1.EnvVar{Name: "SSO_HOSTNAME"},
							},
						},
					},
				},
			},
		},
	}
	cases := []struct {
		Name     string
		Expect   bool
		DC       *v1.DeploymentConfig
		ModifyDC func(config *v1.DeploymentConfig) *v1.DeploymentConfig
	}{
		{
			Name:   "test an upgraded deployment return true",
			DC:     dc,
			Expect: true,
		},
		{
			Name:   "test missing volumes in deployment return false",
			DC:     dc,
			Expect: false,
			ModifyDC: func(config *v1.DeploymentConfig) *v1.DeploymentConfig {
				cp := config.DeepCopy()
				cp.Spec.Template.Spec.Volumes = []corev1.Volume{}
				return cp
			},
		},
		{
			Name:   "test missing env var return false",
			DC:     dc,
			Expect: false,
			ModifyDC: func(config *v1.DeploymentConfig) *v1.DeploymentConfig {
				cp := config.DeepCopy()
				cp.Spec.Template.Spec.Containers[0].Env = []corev1.EnvVar{}
				return cp
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			dc := tc.DC
			if tc.ModifyDC != nil {
				dc = tc.ModifyDC(tc.DC)
			}
			fmt.Println(dc.Spec.Template.Spec.Volumes)
			upgraded := keycloak.DeploymentUpgraded(dc)
			if upgraded != tc.Expect {
				t.Fatalf("expected to get %v but got %v for DeploymentUpgraded ", tc.Expect, upgraded)
			}
		})
	}
}

func TestServiceUpgraded(t *testing.T) {
	cases := []struct {
		Name   string
		Expect bool
		SVC    *corev1.Service
	}{
		{
			Name:   "test service is upgraded",
			Expect: true,
			SVC: &corev1.Service{
				ObjectMeta: v12.ObjectMeta{
					Annotations: map[string]string{"service.alpha.openshift.io/serving-cert-secret-name": "sso-x509-jgroups-secret"},
				},
			},
		},
		{
			Name:   "test service should is not upgraded",
			Expect: false,
			SVC: &corev1.Service{
				ObjectMeta: v12.ObjectMeta{
					Annotations: map[string]string{},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			upgraded := keycloak.ServiceUpgraded(tc.SVC)
			if upgraded != tc.Expect {
				t.Fatalf("Expected to get %v but got %v ", tc.Expect, upgraded)
			}
		})
	}
}

func TestUpgradeDeploymentConfig(t *testing.T) {
	testDC := &v1.DeploymentConfig{
		Spec: v1.DeploymentConfigSpec{
			Triggers: []v1.DeploymentTriggerPolicy{
				v1.DeploymentTriggerPolicy{
					Type: v1.DeploymentTriggerOnImageChange,
					ImageChangeParams: &v1.DeploymentTriggerImageChangeParams{
						From: corev1.ObjectReference{
							Name: "test",
						},
					},
				},
			},
			Template: &corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						corev1.Container{
							Name:         keycloak.SSO_APPLICATION_NAME,
							VolumeMounts: []corev1.VolumeMount{},
							Env: []corev1.EnvVar{
								corev1.EnvVar{
									Name:  "JGROUPS_ENCRYPT_PROTOCOL",
									Value: "test",
								},
							},
						},
					},
				},
			},
		},
	}
	cases := []struct {
		Name     string
		DC       func() *v1.DeploymentConfig
		Validate func(t *testing.T, d *v1.DeploymentConfig)
	}{
		{
			Name: "Test deployment config updated correctly",
			DC: func() *v1.DeploymentConfig {
				return testDC
			},
			Validate: func(t *testing.T, dc *v1.DeploymentConfig) {
				imageTriggerFound := false
				for _, tr := range dc.Spec.Triggers {
					if tr.Type == v1.DeploymentTriggerOnImageChange {
						imageTriggerFound = true
						if tr.ImageChangeParams.From.Name != keycloak.SSO_IMAGE_STREAM {
							t.Fatal("image stream name should be set to ", keycloak.SSO_IMAGE_STREAM, " but is set to ", tr.ImageChangeParams.From.Name)
						}
					}
				}
				if !imageTriggerFound {
					t.Fatal("no image stream trigger found for on image change")
				}
				volumeFound := false
				for _, v := range dc.Spec.Template.Spec.Volumes {
					if v.Name == "sso-x509-jgroups-volume" {
						volumeFound = true
						if v.Secret.SecretName != "sso-x509-jgroups-secret" {
							t.Fatal("expected the volume to be from a secret named sso-x509-jgroups-secret")
						}
					}

				}
				if !volumeFound {
					t.Fatal("did not find new volume after upgrade ")
				}
				containerFound := false
				volumeMountFound := false
				newContainerEnvFound := false

				for _, c := range dc.Spec.Template.Spec.Containers {
					if c.Name == keycloak.SSO_APPLICATION_NAME {
						containerFound = true
						for _, vm := range c.VolumeMounts {
							if vm.Name == "sso-x509-jgroups-volume" {
								volumeMountFound = true
								if !vm.ReadOnly {
									t.Fatal("expected the volume mount ", "sso-x509-jgroups-volume", "to be read only")
								}
								if vm.MountPath != "/etc/x509/jgroups" {
									t.Fatal("epected the mount path to be ", "/etc/x509/jgroups", "but it was "+vm.MountPath)
								}
							}
						}
						for _, ev := range c.Env {
							if ev.Name == "SSO_HOSTNAME" {
								newContainerEnvFound = true
							}
							if ev.Name == "JGROUPS_ENCRYPT_PROTOCOL" {
								t.Fatal("did not expect to find JGROUPS_ENCRYPT_PROTOCOL in the env")
							}
						}
					}
				}
				if !containerFound {
					t.Fatal("expected the ", keycloak.SSO_APPLICATION_NAME, " container but it was not present")
				}
				if !volumeMountFound {
					t.Fatal("expected to find a new volume mount named ", "sso-x509-jgroups-volume", "but found none")
				}
				if !newContainerEnvFound {
					t.Fatal("expected to find a new env var SSO_HOSTNAME but it was missing")
				}

			},
		},
		{
			Name: "test volume and volume only exists once",
			DC: func() *v1.DeploymentConfig {
				dcCopy := testDC.DeepCopy()
				dcCopy.Spec.Template.Spec.Volumes = []corev1.Volume{
					corev1.Volume{
						Name: "sso-x509-jgroups-volume",
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName: "sso-x509-jgroups-secret",
							},
						},
					},
				}
				return dcCopy
			},
			Validate: func(t *testing.T, d *v1.DeploymentConfig) {
				t.Log(d.Spec.Template.Spec.Volumes)
				if len(d.Spec.Template.Spec.Volumes) > 1 {
					t.Fatal("expected only once volume but got ", len(d.Spec.Template.Spec.Volumes))
				}
			},
		},
		{
			Name: "test upgrade and is upgraded",
			DC: func() *v1.DeploymentConfig {
				return testDC
			},
			Validate: func(t *testing.T, d *v1.DeploymentConfig) {
				if !keycloak.DeploymentUpgraded(d) {
					t.Fatal("expected the deployment to be recognised as upgraded")
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			tc.Validate(t, keycloak.UpgradeDeploymentConfig(tc.DC()))
		})
	}
}
