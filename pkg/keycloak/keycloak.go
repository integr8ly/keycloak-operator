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
	scclientset "github.com/kubernetes-incubator/service-catalog/pkg/client/clientset_generated/clientset"
	"github.com/operator-framework/operator-sdk/pkg/sdk"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
)

const (
	KEYCLOAK_SERVICE_NAME = "keycloak"
	KEYCLOAK_PLAN_NAME    = "sharedInstance"
)

type KeycloakRealmPair struct {
	KcRealm  *v1alpha1.KeycloakRealm
	ObjRealm *v1alpha1.KeycloakRealm
}

type Handler struct {
	k8sClient            kubernetes.Interface
	kcClientFactory      KeycloakClientFactory
	serviceCatalogClient scclientset.Interface
}

type ServiceClassExternalMetadata struct {
	ServiceName string `json:"serviceName"`
}

func NewHandler(kcClientFactory KeycloakClientFactory, svcCatalog scclientset.Interface, k8sClient kubernetes.Interface) *Handler {
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

	if kc.GetDeletionTimestamp() != nil && (kc.Status.Phase != v1alpha1.PhaseDeprovisioning && kc.Status.Phase != v1alpha1.PhaseDeprovisioned && kc.Status.Phase != v1alpha1.PhaseDeprovisionFailed) {
		kcCopy.Status.Phase = v1alpha1.PhaseDeprovisioning
		kcCopy.Status.Ready = false

		return sdk.Update(kcCopy)
	}

	logrus.Infof("phase: %v\n", kc.Status.Phase)

	switch kc.Status.Phase {
	case v1alpha1.NoPhase:
		kcCopy.Status.Phase = v1alpha1.PhaseAccepted
		kcCopy.Status.Ready = false

	case v1alpha1.PhaseAccepted:
		if kc.Spec.AdminCredentials == "" {
			adminPwd, err := h.generatePassword()
			if err != nil {
				return err
			}

			adminCredRef, err := h.createAdminCredentials(namespace, "admin", adminPwd)
			if err != nil {
				return err
			}

			kcCopy.Spec.AdminCredentials = adminCredRef.GetName()
			kcCopy.Status.Phase = v1alpha1.PhaseCredentialsPending
		}

	case v1alpha1.PhaseCredentialsPending:
		adminCreds, err := h.k8sClient.CoreV1().Secrets(namespace).Get(kc.Spec.AdminCredentials, metav1.GetOptions{})
		if err != nil {
			return errors.Wrap(err, "failed to get the secret for the admin credentials")
		}
		if adminCreds != nil {
			kcCopy.Status.Phase = v1alpha1.PhaseCredentialsCreated
		}

	case v1alpha1.PhaseCredentialsCreated:
		sc, err := h.getServiceClass()
		if err != nil {
			return err
		}

		adminCreds, err := h.k8sClient.CoreV1().Secrets(namespace).Get(kc.Spec.AdminCredentials, metav1.GetOptions{})
		if err != nil {
			return errors.Wrap(err, "failed to get the secret for the admin credentials")
		}

		decodedParams := map[string]string{}
		for k, v := range adminCreds.Data {
			decodedParams[k] = string(v)
		}

		parameters, err := json.Marshal(decodedParams)
		if err != nil {
			return errors.Wrap(err, "failed to marshal decoded parameters")
		}

		si := h.createServiceInstance(namespace, parameters, *sc)
		serviceInstance, err := h.serviceCatalogClient.Servicecatalog().ServiceInstances(namespace).Create(&si)
		if err != nil {
			kcCopy.Status.Phase = v1alpha1.PhaseFailed
			kcCopy.Status.Message = fmt.Sprintf("failed to create service instance: %v", err)

			updateErr := sdk.Update(kcCopy)
			if updateErr != nil {
				return errors.Wrap(updateErr, fmt.Sprintf("failed to create service instance: %v, failed to update resource", err))
			}

			return errors.Wrap(err, "failed to create service instance")
		}

		kcCopy.Spec.InstanceName = serviceInstance.GetName()
		kcCopy.Spec.InstanceUID = serviceInstance.Spec.ExternalID
		kcCopy.Status.Phase = v1alpha1.PhaseProvisioning

	case v1alpha1.PhaseProvisioning:
		if kc.Spec.InstanceUID == "" {
			kcCopy.Status.Phase = v1alpha1.PhaseFailed
			kcCopy.Status.Message = "instance ID is not defined"

			err := sdk.Update(kcCopy)
			if err != nil {
				return errors.Wrap(err, "instance ID is not defined, failed to update resource")
			}

			return errors.New("instance ID is not defined")
		} else {
			si, err := h.serviceCatalogClient.Servicecatalog().ServiceInstances(namespace).Get(kc.Spec.InstanceName, metav1.GetOptions{})
			if err != nil {
				return errors.Wrap(err, "failed to get service instance")
			}

			if len(si.Status.Conditions) == 0 {
				return nil
			}

			labelSelector := fmt.Sprintf("serviceInstanceID=%s,serviceType=%s", kc.Spec.InstanceUID, "keycloak")
			secretList, err := h.k8sClient.CoreV1().Secrets(kc.Namespace).List(metav1.ListOptions{LabelSelector: labelSelector})
			if err != nil || len(secretList.Items) == 0 {
				return errors.Wrap(err, "error reading admin credentials")
			}

			adminCreds, err := h.k8sClient.CoreV1().Secrets(namespace).Get(kc.Spec.AdminCredentials, metav1.GetOptions{})
			if err != nil {
				return errors.Wrap(err, "failed to get the secret for the admin credentials")
			}

			adminCreds.StringData = map[string]string{}
			adminCreds.StringData["ADMIN_USERNAME"] = string(secretList.Items[0].Data["user_name"])
			adminCreds.StringData["ADMIN_PASSWORD"] = string(secretList.Items[0].Data["user_passwd"])
			adminCreds.StringData["ADMIN_URL"] = string(secretList.Items[0].Data["route_url"])

			_, err = h.k8sClient.CoreV1().Secrets(kc.Namespace).Update(adminCreds)
			if err != nil {
				return errors.Wrap(err, "could not update admin credentials")
			}

			siCondition := si.Status.Conditions[0]
			if siCondition.Type == "Ready" && siCondition.Status == "True" {
				kcCopy.Status.Phase = v1alpha1.PhaseComplete
				kcCopy.Status.Ready = true
			}
		}

	case v1alpha1.PhaseComplete:
		return h.reconcileResources(kcCopy)

	case v1alpha1.PhaseDeprovisioning:
		err := h.deleteKeycloak(kcCopy)
		if err != nil {
			kcCopy.Status.Phase = v1alpha1.PhaseDeprovisionFailed
			kcCopy.Status.Message = fmt.Sprintf("failed to deprovision: %v", err)

			updateErr := sdk.Update(kcCopy)
			if updateErr != nil {
				return errors.Wrap(updateErr, fmt.Sprintf("failed to deprovision instance: %v, failed to update resource", err))
			}

			return errors.Wrap(err, "failed to deprovision")
		}

		kcCopy.Status.Phase = v1alpha1.PhaseDeprovisioned

	case v1alpha1.PhaseDeprovisioned:
		return h.finalizeKeycloak(kcCopy)
	}

	// Only update the Keycloak custom resource if there was a change
	if !reflect.DeepEqual(kc, kcCopy) {
		if err := sdk.Update(kcCopy); err != nil {
			return errors.Wrap(err, "failed to update the keycloak resource")
		}
	}

	return nil
}

func (h *Handler) reconcileResources(kc *v1alpha1.Keycloak) error {
	logrus.Infof("reconciling resources")
	adminCreds, err := h.k8sClient.CoreV1().Secrets(kc.Namespace).Get(kc.Spec.AdminCredentials, metav1.GetOptions{})
	if err != nil {
		return errors.Wrap(err, "failed to get the admin credentials")
	}
	user := string(adminCreds.Data["ADMIN_USERNAME"])
	pass := string(adminCreds.Data["ADMIN_PASSWORD"])
	url := string(adminCreds.Data["ADMIN_URL"])
	logrus.Infof("getting authenticated client for (user: %s, pass: %s, url: %s", user, pass, url)
	kcClient, err := h.kcClientFactory.AuthenticatedClient(*kc, user, pass, url)
	if err != nil {
		return errors.Wrap(err, "failed to get authenticated client for keycloak")
	}

	err = h.reconcileRealms(kc, kcClient)
	if err != nil {
		return errors.Wrap(err, "error reconciling realms")
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

func (h *Handler) createAdminCredentials(namespace, username, password string) (*corev1.Secret, error) {
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
			"ADMIN_USERNAME": username,
			"ADMIN_PASSWORD": password,
		},
		Type: "Opaque",
	}

	if err := sdk.Create(adminCredentialsSecret); err != nil {
		return nil, errors.Wrap(err, "Failed to create secret for the admin credentials")
	}

	return adminCredentialsSecret, nil
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
				ClusterServicePlanExternalName:  KEYCLOAK_PLAN_NAME,
			},
			ClusterServiceClassRef: &v1beta1.ClusterObjectReference{
				Name: sc.Name,
			},
			ClusterServicePlanRef: &v1beta1.ClusterObjectReference{
				Name: KEYCLOAK_PLAN_NAME,
			},
			Parameters: &runtime.RawExtension{Raw: parameters},
		},
	}
}

func (h *Handler) generatePassword() (string, error) {
	generatedPassword, err := uuid.NewRandom()
	if err != nil {
		return "", errors.Wrap(err, "Error generating password")
	}

	return generatedPassword.String(), nil
}

func (h *Handler) deleteKeycloak(kc *v1alpha1.Keycloak) error {
	namespace := kc.ObjectMeta.Namespace

	// Delete keycloak instance
	err := h.serviceCatalogClient.Servicecatalog().ServiceInstances(namespace).Delete(kc.Spec.InstanceUID, &metav1.DeleteOptions{})
	if err != nil && !kerrors.IsNotFound(err) {
		return errors.Wrap(err, "failed to delete service instance")
	}

	// Delete admin creds secret
	err = h.k8sClient.CoreV1().Secrets(namespace).Delete(kc.Spec.AdminCredentials, &metav1.DeleteOptions{})
	if err != nil && !kerrors.IsNotFound(err) {
		return errors.Wrap(err, "failed to delete admin credentials secret")
	}

	return nil
}

func (h *Handler) reconcileRealms(kc *v1alpha1.Keycloak, kcClient KeycloakInterface) error {
	RealmPairsList := map[string]*KeycloakRealmPair{}

	kcRealms, err := kcClient.ListRealms()
	if err != nil {
		return errors.Wrap(err, "error retrieving realms from keycloak")
	}

	for _, realm := range kc.Spec.Realms {
		RealmPairsList[realm.Name] = &KeycloakRealmPair{
			ObjRealm: &realm,
			KcRealm:  kcRealms[realm.Name],
		}
		delete(kcRealms, realm.Name)
	}

	for _, realm := range kcRealms {
		RealmPairsList[realm.Name] = &KeycloakRealmPair{
			KcRealm:  realm,
			ObjRealm: nil,
		}
	}

	for name, realmPair := range RealmPairsList {
		err = h.reconcileRealm(realmPair.KcRealm, realmPair.ObjRealm, kcClient)
		if err != nil {
			return errors.Wrap(err, "error reconciling realm "+name)
		}
	}
	return nil
}

func (h *Handler) reconcileRealm(kcRealm, objRealm *v1alpha1.KeycloakRealm, kcClient KeycloakInterface) error {

	return nil
}

func (h *Handler) reconcileClient(ctx context.Context, wg *sync.WaitGroup, clientDef v1alpha1.KeycloakClient, authenticatedClient KeycloakInterface) error {
	return nil
}

func (h *Handler) reconcileUser(ctx context.Context, wg *sync.WaitGroup, userDef v1alpha1.KeycloakUser, authenticatedClient KeycloakInterface) error {
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
