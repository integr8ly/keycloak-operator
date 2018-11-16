package e2e

import (
	goctx "context"
	"github.com/integr8ly/keycloak-operator/pkg/apis/aerogear/v1alpha1"
	appsv1 "github.com/openshift/client-go/apps/clientset/versioned/typed/apps/v1"
	framework "github.com/operator-framework/operator-sdk/pkg/test"
	"github.com/operator-framework/operator-sdk/pkg/test/e2eutil"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
	"testing"
	"time"
)

func prepare(t *testing.T) *framework.TestCtx {
	ctx := framework.NewTestCtx(t)

	opt := &framework.CleanupOptions{
		TestContext:   ctx,
		RetryInterval: retryInterval,
		Timeout:       timeout,
	}

	err := ctx.InitializeClusterResources(opt)
	if err != nil {
		t.Fatalf("Failed to initialize test context: %v", err)
	}

	ns, err := ctx.GetNamespace()
	if err != nil {
		t.Fatalf("Failed to get context namespace: %v", err)
	}

	globalVars := framework.Global

	err = e2eutil.WaitForDeployment(t, globalVars.KubeClient, ns, "keycloak-operator", 1, retryInterval, timeout)
	if err != nil {
		t.Fatalf("Operator deployment failed: %v", err)
	}

	return ctx
}

func register() error {
	var err error

	keycloakList := &v1alpha1.KeycloakList{
		TypeMeta: metav1.TypeMeta{
			Kind:       v1alpha1.KeycloakKind,
			APIVersion: v1alpha1.Group + "/" + v1alpha1.Version,
		},
	}
	err = framework.AddToFrameworkScheme(v1alpha1.AddToScheme, keycloakList)
	if err != nil {
		return err
	}

	keycloak := &v1alpha1.Keycloak{
		TypeMeta: metav1.TypeMeta{
			Kind:       v1alpha1.KeycloakKind,
			APIVersion: v1alpha1.Group + "/" + v1alpha1.Version,
		},
	}
	err = framework.AddToFrameworkScheme(v1alpha1.AddToScheme, keycloak)
	if err != nil {
		return err
	}

	keycloakRealmList := &v1alpha1.KeycloakRealmList{
		TypeMeta: metav1.TypeMeta{
			Kind:       v1alpha1.KeycloakRealmKind,
			APIVersion: v1alpha1.Group + "/" + v1alpha1.Version,
		},
	}
	err = framework.AddToFrameworkScheme(v1alpha1.AddToScheme, keycloakRealmList)
	if err != nil {
		return err
	}

	keycloakRealm := &v1alpha1.KeycloakRealm{
		TypeMeta: metav1.TypeMeta{
			Kind:       v1alpha1.KeycloakRealmKind,
			APIVersion: v1alpha1.Group + "/" + v1alpha1.Version,
		},
	}
	err = framework.AddToFrameworkScheme(v1alpha1.AddToScheme, keycloakRealm)
	if err != nil {
		return err
	}

	return nil
}

func cleanupOpts(ctx *framework.TestCtx) *framework.CleanupOptions {
	return &framework.CleanupOptions{
		TestContext:   ctx,
		Timeout:       timeout,
		RetryInterval: retryInterval,
	}
}

func doDeployment(f *framework.Framework, ctx *framework.TestCtx, keycloakCr *v1alpha1.Keycloak, realmCr *v1alpha1.KeycloakRealm) error {

	err := f.Client.Create(goctx.TODO(), realmCr, cleanupOpts(ctx))
	if err != nil {
		return err
	}

	err = f.Client.Create(goctx.TODO(), keycloakCr, cleanupOpts(ctx))
	if err != nil {
		return err
	}

	return nil
}

func waitForDC(config *rest.Config, namespace, name string, retryInterval, timeout time.Duration) error {
	kubeApps, _ := appsv1.NewForConfig(config)
	err := wait.Poll(retryInterval, timeout, func() (done bool, err error) {
		dc, err := kubeApps.DeploymentConfigs(namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}

		if dc.Status.Replicas == dc.Status.ReadyReplicas {
			return true, nil
		}

		return false, nil
	})

	return err
}

func validateKeycloakDeployment(f *framework.Framework, ns string, retryInterval, timeout time.Duration) error {
	var err error

	err = waitForDC(f.KubeConfig, ns, "sso-postgresql", retryInterval, timeout)
	if err != nil {
		return err
	}

	err = waitForDC(f.KubeConfig, ns, "sso", retryInterval, timeout)
	if err != nil {
		return err
	}

	return nil
}

func doRealmDeployment(f *framework.Framework, ctx *framework.TestCtx, cr *v1alpha1.KeycloakRealm) error {
	err := f.Client.Create(goctx.TODO(), cr, cleanupOpts(ctx))
	if err != nil {
		return err
	}

	return nil
}

func waitForRealm(f *framework.Framework, cr *v1alpha1.KeycloakRealm, retryInterval, timeout time.Duration) error {
	realmKey := types.NamespacedName{
		Name:      cr.Name,
		Namespace: cr.Namespace,
	}

	err := wait.Poll(retryInterval, timeout, func() (done bool, err error) {
		ref := &v1alpha1.KeycloakRealm{
			TypeMeta: metav1.TypeMeta{
				Kind:       v1alpha1.KeycloakRealmKind,
				APIVersion: v1alpha1.Group + "/" + v1alpha1.Version,
			},
		}

		err = f.Client.Get(goctx.TODO(), realmKey, ref)
		if err != nil {
			logrus.Infof("Error retrieving CR: %v", err)
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}

		if ref.Status.Phase == v1alpha1.PhaseReconcile {
			return true, nil
		}

		return false, nil
	})

	return err
}

func validateRealmDeployment(f *framework.Framework, cr *v1alpha1.KeycloakRealm, retryInterval, timeout time.Duration) error {
	var err error

	err = waitForRealm(f, cr, retryInterval, timeout)
	if err != nil {
		return err
	}

	return nil
}
