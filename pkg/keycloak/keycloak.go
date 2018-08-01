package keycloak

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"

	"github.com/aerogear/keycloak-operator/pkg/apis/aerogear/v1alpha1"
	"github.com/google/uuid"
	sc "github.com/kubernetes-incubator/service-catalog/pkg/api/meta"
	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1beta1"
	sc "github.com/kubernetes-incubator/service-catalog/pkg/client/clientset_generated/clientset"
	"github.com/operator-framework/operator-sdk/pkg/sdk"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
)

const KEYCLOAK_SERVICE_NAME = "keycloak"

type Handler struct {
	k8sClient            kubernetes.Interface
	kcClientFactory      KeycloakClientFactory
	serviceCatalogClient sc.Interface
}

type ServiceClassExternalMetadata struct {
	ServiceName string `json:"serviceName"`
}

func NewHandler(kcClientFactory KeycloakClientFactory, svcCatalog sc.Interface, k8sClient kubernetes.Interface) *Handler {
	return &Handler{
		kcClientFactory:      kcClientFactory,
		serviceCatalogClient: svcCatalog,
		k8sClient:            k8sClient,
	}
}

func (h *Handler) Handle(ctx context.Context, event sdk.Event) error {
	kc := event.Object.(*v1alpha1.Keycloak)
	kcCopy := kc.DeepCopy()
	namespace := kc.ObjectMeta.Namespace

	if kc.Status.Phase == v1alpha1.NoPhase {
		kcCopy.Status.Phase = v1alpha1.PhaseAccepted
		kcCopy.Status.Ready = false
	}

	if kc.Status.Phase == v1alpha1.PhaseAccepted {
		sc, err := h.getServiceClass()
		if err != nil {
			return err
		}

		if kc.Spec.AdminCredentials == "" {
			adminCredRef, err := h.createAdminCredentials(namespace)
			if err != nil {
				return err
			}

			kcCopy.Spec.AdminCredentials = adminCredRef
		} else {
			adminCreds, err := h.k8sClient.CoreV1().Secrets(namespace).Get(kc.Spec.AdminCredentials, metav1.GetOptions{})
			if err != nil {
				return errors.Wrap(err, "Failed to get the secret for the admin credentials")
			}

			decodedParams := map[string]string{}
			for k, v := range adminCreds.Data {
				decodedParams[k] = string(v)
			}

			parameters, err := json.Marshal(decodedParams)
			if err != nil {
				fmt.Println(err)
			}

			si := h.createServiceInstance(namespace, parameters, *sc)
			serviceInstance, err := h.serviceCatalogClient.Servicecatalog().ServiceInstances(namespace).Create(&si)
			if err != nil {
				return errors.Wrap(err, "Failed to create service instance")
			}

			kcCopy.Spec.InstanceID = serviceInstance.GetName()
			kcCopy.Status.Phase = v1alpha1.PhaseProvisioning
		}
	}

	if kc.Status.Phase == v1alpha1.PhaseProvisioning {
		if kc.Spec.InstanceID == "" {
			return errors.New("Instance ID is not defined")
		} else {
			si, err := h.serviceCatalogClient.Servicecatalog().ServiceInstances(namespace).Get(kc.Spec.InstanceID, metav1.GetOptions{})
			if err != nil {
				return errors.Wrap(err, "Failed to get service instance")
			}

			if len(si.Status.Conditions) == 0 {
				return nil
			}

			siCondition := si.Status.Conditions[0]
			if siCondition.Type == "Ready" && siCondition.Status == "True" {
				kcCopy.Status.Phase = v1alpha1.PhaseComplete
			}
		}
	}

	if kc.Status.Phase == v1alpha1.PhaseComplete {
		kcCopy.Status.Ready = true
	}

	// set up authenticated client
	authenticatedClient, err := h.kcClientFactory.AuthenticatedClient(*kcCopy)
	if err != nil {
		return errors.Wrap(err, "failed to get authenticated client for keycloak")
	}
	// hand of each realm to reconcile realm may want to make async to avoid blocking
	for _, r := range kcCopy.Spec.Realms {
		if err := h.reconcileRealm(ctx, r, authenticatedClient); err != nil {
			return errors.Wrap(err, "failed to reconcile realm "+r.Name)
		}
	}

	if event.Deleted {
		return nil
	}

	if kcCopy.GetDeletionTimestamp() != nil {
		return h.finalizeKeycloak(kcCopy)
	}

	if kc.Status.Phase == v1alpha1.PhaseAccepted {
		logrus.Info("not doing anything as this resource is already being worked on")
		return nil
	}

	// Only update the Keycloak custom resource if there was a change
	if !reflect.DeepEqual(kc, kcCopy) {
		if err := sdk.Update(kcCopy); err != nil {
			return errors.Wrap(err, "failed to update the keycloak resource")
		}
	}

	return nil
}

func (h *Handler) getServiceClass() (*v1beta1.ClusterServiceClass, error) {
	var svcClassExtMetadata ServiceClassExternalMetadata

	csc, err := h.serviceCatalogClient.Servicecatalog().ClusterServiceClasses().List(metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get service classes")
	}

	for _, sc := range csc.Items {
		externalMetadata, err := sc.Spec.ExternalMetadata.MarshalJSON()
		if err != nil {
			return nil, errors.Wrap(err, "Failed to marshal the service class external metadata")
		}

		if err := json.Unmarshal(externalMetadata, &svcClassExtMetadata); err != nil {
			return nil, errors.Wrap(err, "Failed to unmarshal the service class external metadata to a JSON object")
		}

		// NOTE: This may need to be improved in order to abstract it for the shared service lib
		if svcClassExtMetadata.ServiceName == KEYCLOAK_SERVICE_NAME {
			return &sc, nil
		}
	}

	return nil, errors.Wrap(err, "Failed to find service class")
}

func (h *Handler) createAdminCredentials(namespace string) (string, error) {
	adminCredentialsSecret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    namespace,
			GenerateName: "keycloak-admin-cred-",
		},
		StringData: map[string]string{
			"ADMIN_USERNAME": "admin",
			"ADMIN_PASSWORD": h.generatePassword(),
		},
		Type: "Opaque",
	}

	if err := sdk.Create(adminCredentialsSecret); err != nil {
		return "", errors.Wrap(err, "Failed to create secret for the admin credentials")
	}

	return adminCredentialsSecret.GetName(), nil
}

func (h *Handler) createServiceInstance(namespace string, parameters []byte, sc v1beta1.ClusterServiceClass) v1beta1.ServiceInstance {
	return v1beta1.ServiceInstance{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "servicecatalog.k8s.io/v1beta1",
			Kind:       "ServiceInstance",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    namespace,
			GenerateName: sc.Spec.ExternalName + "-",
		},
		Spec: v1beta1.ServiceInstanceSpec{
			PlanReference: v1beta1.PlanReference{
				ClusterServiceClassExternalName: sc.Spec.ExternalName,
			},
			ClusterServiceClassRef: &v1beta1.ClusterObjectReference{
				Name: sc.Name,
			},
			ClusterServicePlanRef: &v1beta1.ClusterObjectReference{
				Name: "default",
			},
			Parameters: &runtime.RawExtension{Raw: parameters},
		},
	}
}

func (h *Handler) generatePassword() string {
	generatedPassword, err := uuid.NewRandom()
	if err != nil {
		fmt.Println("Error generating password, setting to default value", err)
		return "admin"
	}

	return generatedPassword.String()
}

func (h *Handler) reconcileRealm(ctx context.Context, realm v1alpha1.KeycloakRealm, authenticatedClient KeycloakInterface) error {
	rc := realm.DeepCopy()
	// check does realm exist
	exists, err := authenticatedClient.DoesRealmExist(rc.Name)
	if err != nil {
		return err
	}
	if !exists {
		// create realm
	}

	return nil
}

func (h *Handler) reconcileClient(ctx context.Context, wg *sync.WaitGroup, clientDef v1alpha1.KeycloakClient, authenticatedClient KeycloakInterface) error {
	return nil
}

func (h *Handler) reconcileUser(ctx context.Context, wg *sync.WaitGroup, userDef v1alpha1.KeycloakUser, authenticatedClient KeycloakInterface) error {
	return nil
}

func (h *Handler) deleteKeycloak(kc *v1alpha1.Keycloak) error {
	return nil
}

func (sh *Handler) finalizeKeycloak(kc *v1alpha1.Keycloak) error {
	sc.RemoveFinalizer(kc, v1alpha1.KeycloakFinalizer)
	err := sdk.Update(kc)
	if err != nil {
		logrus.Errorf("error updating resource finalizer: %v", err)
		return err
	}
	return nil
}

func (h *Handler) GVK() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Version: v1alpha1.Version,
		Group:   v1alpha1.Group,
		Kind:    v1alpha1.KeycloakKind,
	}
}
