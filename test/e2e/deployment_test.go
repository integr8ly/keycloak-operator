package e2e

import (
	"github.com/integr8ly/keycloak-operator/pkg/apis/aerogear/v1alpha1"
	framework "github.com/operator-framework/operator-sdk/pkg/test"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func TestRealmCreation(t *testing.T) {
	var err error
	t.Parallel()

	ctx := prepare(t)
	defer ctx.Cleanup()

	err = register()
	if err != nil {
		t.Fatalf("Failed to register crd scheme: %v", err)
	}

	ns, err := ctx.GetNamespace()
	if err != nil {
		t.Fatalf("Failed to retrieve namespace: %v", err)
	}

	keycloakCr := &v1alpha1.Keycloak{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Keycloak",
			APIVersion: "aerogear.org/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "keycloak-test",
			Namespace: ns,
		},
		Spec: v1alpha1.KeycloakSpec{
			Version:          "4.1.0",
			AdminCredentials: "credential-keycloak-test",
		},
	}

	realmCr := &v1alpha1.KeycloakRealm{
		TypeMeta: metav1.TypeMeta{
			Kind:       "KeycloakRealm",
			APIVersion: "aerogear.org/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "openshift",
			Namespace: ns,
		},
		Spec: v1alpha1.KeycloakRealmSpec{
			KeycloakApiRealm: &v1alpha1.KeycloakApiRealm{
				ID:          "openshift",
				Realm:       "openshift",
				DisplayName: "openshift",
				Enabled:     true,
				CreateOnly:  true,
			},
		},
	}

	err = doDeployment(framework.Global, ctx, keycloakCr, realmCr)
	if err != nil {
		t.Fatalf("Failed to deploy keycloak: %v", err)
	}

	//adding extra timeout time because sso pod takes a while to be ready
	err = validateKeycloakDeployment(framework.Global, ns, retryInterval, timeout*10)
	if err != nil {
		t.Fatalf("Failed to validate keycloak deployment: %v", err)
	}

	err = validateRealmDeployment(framework.Global, realmCr, retryInterval, timeout*10)
	if err != nil {
		t.Fatalf("Failed to deploy keycloak realm: %v", err)
	}
	logrus.Info("deployment end")
}
