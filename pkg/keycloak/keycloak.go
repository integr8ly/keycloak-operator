package keycloak

import (
	"context"
	"os"

	"strings"

	"github.com/google/uuid"
	"github.com/integr8ly/keycloak-operator/pkg/apis/aerogear/v1alpha1"
	apps "github.com/openshift/client-go/apps/clientset/versioned/typed/apps/v1"
	routev1 "github.com/openshift/client-go/route/clientset/versioned/typed/route/v1"
	"github.com/operator-framework/operator-sdk/pkg/k8sclient"
	"github.com/operator-framework/operator-sdk/pkg/sdk"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	SSO_TEMPLATE_NAME         = "sso72-x509-postgresql-persistent.json"
	SSO_ROUTE_NAME            = "sso"
	SSO_APPLICATION_NAME      = "sso"
	SSO_TEMPLATE_PATH         = "deploy/template"
	SSO_TEMPLATE_PATH_ENV_VAR = "TEMPLATE_DIR"
	SSO_VERSION               = "7.2.2"
)

//go:generate moq -out sdkCruder_moq.go . SdkCruder

type SdkCruder interface {
	Create(object sdk.Object) error
	Update(object sdk.Object) error
	Delete(object sdk.Object, opts ...sdk.DeleteOption) error
	Get(object sdk.Object, opts ...sdk.GetOption) error
	List(namespace string, into sdk.Object, opts ...sdk.ListOption) error
}

type CredentialService interface {
	CreateCredentialSecret(namespace, kcName, realm string, data map[string][]byte) (*corev1.Secret, error)
	GetCredentialSecret(kcName, realm, namespace string) (*corev1.Secret, error)
	DeleteCredentialSecret(kcName, realm, namespace string) error
	GeneratePassword() (string, error)
}

type Reconciler struct {
	cfg             v1alpha1.Config
	k8sClient       kubernetes.Interface
	kcClientFactory KeycloakClientFactory
	kubeconfig      *rest.Config
	defaultClients  map[string]struct{}
	sdkCrud         SdkCruder
	phaseHandler    *phaseHandler
}

func NewReconciler(kcClientFactory KeycloakClientFactory, k8client kubernetes.Interface, cruder SdkCruder) *Reconciler {
	kcDefaultClients := []string{"account", "admin-cli", "broker", "realm-management", "security-admin-console"}
	set := make(map[string]struct{}, len(kcDefaultClients))
	for _, s := range kcDefaultClients {
		set[s] = struct{}{}
	}
	// todo move out and stop ignoring error
	kubeconfig := k8sclient.GetKubeConfig()
	routeClient, _ := routev1.NewForConfig(kubeconfig)
	dcClient, _ := apps.NewForConfig(kubeconfig)
	return &Reconciler{
		kcClientFactory: kcClientFactory,
		k8sClient:       k8client,
		defaultClients:  set,
		kubeconfig:      kubeconfig,
		sdkCrud:         cruder,
		phaseHandler:    NewPhaseHandler(k8client, routeClient, dcClient, k8sclient.GetResourceClient),
	}
}

func (h *Reconciler) Handle(ctx context.Context, object interface{}, deleted bool) error {

	if deleted {
		return nil
	}

	kc, ok := object.(*v1alpha1.Keycloak)
	if !ok {
		return errors.New("error converting object to keycloak realm")
	}

	//TODO use the ctx to allow us to cancel any ongoing requests especially when the resource is deleted
	kcCopy := kc.DeepCopy()
	logrus.Infof("Keycloak: %v, Phase: %v", kc.Name, kc.Status.Phase)
	if kc.GetDeletionTimestamp() != nil {
		//update reliant realms
		for _, ns := range strings.Split(os.Getenv("CONSUMER_NAMESPACES"), ";") {
			list := &v1alpha1.KeycloakRealmList{
				TypeMeta: metav1.TypeMeta{
					Kind:       "keycloakrealm",
					APIVersion: v1alpha1.Group + "/" + v1alpha1.Version,
				},
			}
			err := h.sdkCrud.List(ns, list)
			if err != nil {
				continue
			}

			for _, realm := range list.Items {
				if realm.Status.KeycloakName == kc.Name {
					realm.Status.Phase = v1alpha1.PhaseInstanceDeprovisioned
					realm.Status.Message = "The owner keycloak instance has been deprovisioned, this realm will no longer be reconciled"
					h.sdkCrud.Update(&realm)
				}
			}
		}

		kcState, err := h.phaseHandler.Deprovision(kcCopy)
		if err != nil {
			return errors.Wrap(err, "failed to deprovision")
		}

		return h.sdkCrud.Update(kcState)
	}

	switch kc.Status.Phase {
	case v1alpha1.NoPhase:
		kcState, err := h.phaseHandler.Initialise(kc)
		if err != nil {
			return errors.Wrap(err, "failed to init resource")
		}
		return h.sdkCrud.Update(kcState)

	case v1alpha1.PhaseAccepted:
		kcState, err := h.phaseHandler.Accepted(kc)
		if err != nil {
			return errors.Wrap(err, "phase accepted failed")
		}
		return h.sdkCrud.Update(kcState)
	case v1alpha1.PhaseProvision:

		kcState, err := h.phaseHandler.Provision(kc)
		if err != nil {
			return errors.Wrap(err, "phase provision failed")
		}
		return h.sdkCrud.Update(kcState)
	case v1alpha1.PhaseReconcile:
		kcState, err := h.phaseHandler.Reconcile(kc)
		if err != nil {
			return errors.Wrap(err, "phase provisioned failed")
		}
		return h.sdkCrud.Update(kcState)
	case v1alpha1.PhaseComplete:
		return nil
	}
	return nil
}

func (h *Reconciler) GVK() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Version: v1alpha1.Version,
		Group:   v1alpha1.Group,
		Kind:    v1alpha1.KeycloakKind,
	}
}

func GeneratePassword() (string, error) {
	generatedPassword, err := uuid.NewRandom()
	if err != nil {
		return "", errors.Wrap(err, "error generating password")
	}
	return strings.Replace(generatedPassword.String(), "-", "", 10), nil
}
