package keycloak

import (
	"context"
	"encoding/json"
	"fmt"

	"reflect"

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

type Handler struct {
	cfg                  v1alpha1.Config
	k8sClient            kubernetes.Interface
	kcClientFactory      KeycloakClientFactory
	serviceCatalogClient scclientset.Interface
	defaultClients       map[string]struct{}
}

type ServiceClassExternalMetadata struct {
	ServiceName string `json:"serviceName"`
}

func NewHandler(cfg v1alpha1.Config, kcClientFactory KeycloakClientFactory, svcCatalog scclientset.Interface, k8sClient kubernetes.Interface) *Handler {
	kcDefaultClients := []string{"account", "admin-cli", "broker", "realm-management", "security-admin-console"}
	set := make(map[string]struct{}, len(kcDefaultClients))
	for _, s := range kcDefaultClients {
		set[s] = struct{}{}
	}

	return &Handler{
		cfg:                  cfg,
		kcClientFactory:      kcClientFactory,
		serviceCatalogClient: svcCatalog,
		k8sClient:            k8sClient,
		defaultClients:       set,
	}
}

func (h *Handler) Handle(ctx context.Context, event sdk.Event) error {
	logrus.Debug("handling object ", event.Object.GetObjectKind().GroupVersionKind().String())

	kc := event.Object.(*v1alpha1.Keycloak)
	kcCopy := kc.DeepCopy()
	namespace := kc.ObjectMeta.Namespace

	logrus.Debugf("Keycloak: %v, Phase: %v", kc.Name, kc.Status.Phase)

	if event.Deleted {
		return nil
	}

	if kc.GetDeletionTimestamp() != nil && (kc.Status.Phase != v1alpha1.PhaseDeprovisioning && kc.Status.Phase != v1alpha1.PhaseDeprovisioned && kc.Status.Phase != v1alpha1.PhaseDeprovisionFailed) {
		kcCopy.Status.Phase = v1alpha1.PhaseDeprovisioning
		kcCopy.Status.Ready = false

		return sdk.Update(kcCopy)
	}

	switch kc.Status.Phase {
	case v1alpha1.NoPhase:
		return h.initKeycloak(kcCopy)

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
		svcClass, err := h.getServiceClass()
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

		si := h.createServiceInstance(namespace, parameters, *svcClass)
		serviceInstance, err := h.serviceCatalogClient.ServicecatalogV1beta1().ServiceInstances(namespace).Create(&si)
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
		}

		si, err := h.serviceCatalogClient.ServicecatalogV1beta1().ServiceInstances(namespace).Get(kc.Spec.InstanceName, metav1.GetOptions{})
		if err != nil {
			return errors.Wrap(err, "failed to get service instance")
		}

		if len(si.Status.Conditions) == 0 {
			return nil
		}

		labelSelector := fmt.Sprintf("serviceInstanceID=%s,serviceType=%s", kc.Spec.InstanceUID, "keycloak")
		secretList, err := h.k8sClient.CoreV1().Secrets(kc.Namespace).List(metav1.ListOptions{LabelSelector: labelSelector})
		if err != nil {
			return errors.Wrap(err, "error reading admin credentials")
		}

		if len(secretList.Items) == 0 {
			logrus.Debug("keycloak service credentials not found")
			return nil
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

	case v1alpha1.PhaseComplete:
		err := h.reconcileResources(kcCopy)
		if err != nil {
			return errors.Wrap(err, "could not reconcile resources")
		}

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
	logrus.Infof("reconcile resources (%v)", kc.Name)
	adminCreds, err := h.k8sClient.CoreV1().Secrets(kc.Namespace).Get(kc.Spec.AdminCredentials, metav1.GetOptions{})
	if err != nil {
		return errors.Wrap(err, "failed to get the admin credentials")
	}
	user := string(adminCreds.Data["ADMIN_USERNAME"])
	pass := string(adminCreds.Data["ADMIN_PASSWORD"])
	url := string(adminCreds.Data["ADMIN_URL"])
	logrus.Debugf("getting authenticated client for (user: %s, pass: %s, url: %s", user, pass, url)

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

	cscs, err := h.serviceCatalogClient.ServicecatalogV1beta1().ClusterServiceClasses().List(metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to get service classes")
	}

	for _, csc := range cscs.Items {
		externalMetadata, err := csc.Spec.ExternalMetadata.MarshalJSON()
		if err != nil {
			return nil, errors.Wrap(err, "failed to marshal the service class external metadata")
		}

		if err := json.Unmarshal(externalMetadata, &svcClassExtMetadata); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal the service class external metadata to a JSON object")
		}

		// NOTE: This may need to be improved in order to abstract it for the shared service lib
		if svcClassExtMetadata.ServiceName == KEYCLOAK_SERVICE_NAME {
			return &csc, nil
		}
	}

	return nil, errors.Wrap(err, "failed to find service class")
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
		return nil, errors.Wrap(err, "failed to create secret for the admin credentials")
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
		return "", errors.Wrap(err, "error generating password")
	}

	return generatedPassword.String(), nil
}

func (h *Handler) deleteKeycloak(kc *v1alpha1.Keycloak) error {
	namespace := kc.ObjectMeta.Namespace

	// Delete keycloak instance
	err := h.serviceCatalogClient.ServicecatalogV1beta1().ServiceInstances(namespace).Delete(kc.Spec.InstanceName, &metav1.DeleteOptions{})
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
	logrus.Infof("reconcile realms (%v)", kc.Name)

	specRealms := kc.Spec.Realms
	kcRealms, err := kcClient.ListRealms()
	if err != nil {
		return errors.Wrap(err, "error retrieving realms from keycloak")
	}

	kcRealmMap := map[string]*v1alpha1.KeycloakRealm{}
	for i := range kcRealms {
		logrus.Debugf("kc realm %v", kcRealms[i].ID)
		kcRealmMap[kcRealms[i].ID] = kcRealms[i]
	}

	for i := range specRealms {
		logrus.Debugf("spec realm %v", specRealms[i].ID)
		err = h.reconcileRealm(kcRealmMap[specRealms[i].ID], &specRealms[i], kcClient)
		//This should try and reconcile all realms rather throwing an error on the first failure
		if err != nil {
			return errors.Wrapf(err, "error reconciling realm: %v", specRealms[i].ID)
		}
	}

	return nil
}

func (h *Handler) reconcileRealm(kcRealm, specRealm *v1alpha1.KeycloakRealm, kcClient KeycloakInterface) error {
	logrus.Infof("reconcile realm (%v)", specRealm.ID)

	if kcRealm == nil {
		logrus.Debugf("create realm: %v", specRealm.ID)
		err := kcClient.CreateRealm(specRealm)
		if err != nil {
			logrus.Errorf("error creating realm %v", specRealm.ID)
			return errors.Wrap(err, "error creating keycloak realm")
		}
	} else {
		if h.cfg.SyncResources {
			logrus.Debugf("sync realm %v", specRealm.ID)
			if !resourcesEqual(kcRealm, specRealm) {
				err := kcClient.UpdateRealm(specRealm)
				if err != nil {
					logrus.Errorf("error updating realm %v", specRealm.ID)
					return errors.Wrap(err, "error updating keycloak realm")
				}
			}

			h.reconcileClients(kcClient, specRealm)
			h.reconcileUsers(kcClient, specRealm)
			h.reconcileIdentityProviders(kcClient, specRealm)
		}
	}

	return nil
}

func (h *Handler) reconcileClients(kcClient KeycloakInterface, specRealm *v1alpha1.KeycloakRealm) error {
	logrus.Infof("reconcile clients (%v)", specRealm.ID)

	clients, err := kcClient.ListClients(specRealm.Realm)
	if err != nil {
		return err
	}

	kcClients := map[string]*v1alpha1.KeycloakClient{}
	for i := range clients {
		kcClients[clients[i].ClientID] = clients[i]
	}

	clientPairsList := map[string]*v1alpha1.KeycloakClientPair{}
	for i := range specRealm.Clients {
		client := &specRealm.Clients[i]
		clientPairsList[client.ClientID] = &v1alpha1.KeycloakClientPair{
			SpecClient: client,
			KcClient:   kcClients[client.ClientID],
		}
		delete(kcClients, client.ClientID)
	}

	for i := range kcClients {
		client := kcClients[i]
		clientPairsList[client.ClientID] = &v1alpha1.KeycloakClientPair{
			KcClient:   client,
			SpecClient: nil,
		}
	}

	for i := range clientPairsList {
		err := h.reconcileClient(clientPairsList[i].KcClient, clientPairsList[i].SpecClient, specRealm.Realm, kcClient)
		if err != nil {
			return err
		}
	}

	return nil
}

func (h *Handler) reconcileClient(kcClient, specClient *v1alpha1.KeycloakClient, realmName string, authenticatedClient KeycloakInterface) error {
	if specClient == nil && !h.isDefaultClient(kcClient.ClientID) {
		logrus.Debugf("Deleting client %s in realm: %s", kcClient.ClientID, realmName)
		err := authenticatedClient.DeleteClient(kcClient.ID, realmName)
		if err != nil {
			return err
		}
	} else if kcClient == nil {
		logrus.Debugf("Creating client %s in realm: %s", specClient.ClientID, realmName)
		err := authenticatedClient.CreateClient(specClient, realmName)
		if err != nil {
			return err
		}
	} else {
		if !resourcesEqual(kcClient, specClient) && !h.isDefaultClient(kcClient.ClientID) {
			logrus.Debugf("Updating client %s in realm: %s", kcClient.ClientID, realmName)
			specClient.ID = kcClient.ID
			err := authenticatedClient.UpdateClient(specClient, realmName)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// resourcesEqual is used to tell whether or not a keycloak resource matches its spec
// TODO: this could be improved as it doesn't currently work as expected for users and clients
func resourcesEqual(obj1, obj2 T) bool {
	return reflect.DeepEqual(obj1, obj2)
}

func (h *Handler) isDefaultClient(client string) bool {
	_, ok := h.defaultClients[client]
	return ok
}

func (h *Handler) reconcileUsers(kcClient KeycloakInterface, specRealm *v1alpha1.KeycloakRealm) error {
	logrus.Info("reconciling users")

	users, err := kcClient.ListUsers(specRealm.Realm)
	if err != nil {
		return err
	}

	kcUsers := map[string]*v1alpha1.KeycloakUser{}
	for i := range users {
		kcUsers[users[i].UserName] = users[i]
	}

	userPairsList := map[string]*v1alpha1.KeycloakUserPair{}
	for i := range specRealm.Users {
		user := &specRealm.Users[i]
		userPairsList[user.UserName] = &v1alpha1.KeycloakUserPair{
			SpecUser: user,
			KcUser:   kcUsers[user.UserName],
		}
	}

	for i := range userPairsList {
		err := h.reconcileUser(userPairsList[i].KcUser, userPairsList[i].SpecUser, specRealm.Realm, kcClient)
		if err != nil {
			return err
		}
	}

	return nil
}

func (h *Handler) reconcileUser(kcUser, specUser *v1alpha1.KeycloakUser, realmName string, authenticatedClient KeycloakInterface) error {
	if kcUser == nil {
		logrus.Debugf("Creating user %s, %s in realm: %s", specUser.ID, specUser.UserName, realmName)
		err := authenticatedClient.CreateUser(specUser, realmName)
		if err != nil {
			return err
		}
	} else {
		if !resourcesEqual(kcUser, specUser) {
			logrus.Debugf("Updating user %s, %s in realm: %s", kcUser.ID, kcUser.UserName, realmName)
			specUser.ID = kcUser.ID
			err := authenticatedClient.UpdateUser(specUser, realmName)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (h *Handler) reconcileIdentityProviders(kcClient KeycloakInterface, specRealm *v1alpha1.KeycloakRealm) error {
	logrus.Infof("reconciling identity providers (%v)", specRealm.ID)

	identityProviders, err := kcClient.ListIdentityProviders(specRealm.Realm)
	if err != nil {
		return err
	}

	kcIdentityProviders := map[string]*v1alpha1.KeycloakIdentityProvider{}
	for i := range identityProviders {
		kcIdentityProviders[identityProviders[i].Alias] = identityProviders[i]
	}

	identityProviderPairsList := map[string]*v1alpha1.KeycloakIdentityProviderPair{}
	for i := range specRealm.IdentityProviders {
		identityProvider := &specRealm.IdentityProviders[i]
		identityProviderPairsList[identityProvider.Alias] = &v1alpha1.KeycloakIdentityProviderPair{
			SpecIdentityProvider: identityProvider,
			KcIdentityProvider:   kcIdentityProviders[identityProvider.Alias],
		}
		delete(kcIdentityProviders, identityProvider.Alias)
	}

	for i := range kcIdentityProviders {
		identityProvider := kcIdentityProviders[i]
		identityProviderPairsList[identityProvider.Alias] = &v1alpha1.KeycloakIdentityProviderPair{
			KcIdentityProvider:   identityProvider,
			SpecIdentityProvider: nil,
		}
	}

	for i := range identityProviderPairsList {
		err := h.reconcileIdentityProvider(identityProviderPairsList[i].KcIdentityProvider, identityProviderPairsList[i].SpecIdentityProvider, specRealm.Realm, kcClient)
		if err != nil {
			return err
		}
	}

	return nil
}

func (h *Handler) reconcileIdentityProvider(kcIdentityProvider, specIdentityProvider *v1alpha1.KeycloakIdentityProvider, realmName string, authenticatedClient KeycloakInterface) error {
	if specIdentityProvider == nil {
		logrus.Debugf("Deleting identity provider %s in realm: %s", kcIdentityProvider.Alias, realmName)
		err := authenticatedClient.DeleteIdentityProvider(kcIdentityProvider.Alias, realmName)
		if err != nil {
			return err
		}
	} else if kcIdentityProvider == nil {
		logrus.Debugf("Creating identity provider %s in realm: %s", specIdentityProvider.Alias, realmName)
		err := authenticatedClient.CreateIdentityProvider(specIdentityProvider, realmName)
		if err != nil {
			return err
		}
	} else {
		//The API doesn't return the secret, so in order to stop in never being equal we just set it to the spec version
		kcIdentityProvider.Config["clientSecret"] = specIdentityProvider.Config["clientSecret"]
		//Ensure the internalID is set on the spec object, this is required for update requests to succeed
		specIdentityProvider.InternalID = kcIdentityProvider.InternalID
		if !resourcesEqual(kcIdentityProvider, specIdentityProvider) {
			logrus.Debugf("Updating identity provider %s in realm: %s", kcIdentityProvider.Alias, realmName)
			err := authenticatedClient.UpdateIdentityProvider(specIdentityProvider, realmName)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (h *Handler) initKeycloak(kc *v1alpha1.Keycloak) error {
	logrus.Infof("initialise keycloak: %v", kc.Name)
	sc.AddFinalizer(kc, v1alpha1.KeycloakFinalizer)
	kc.Status.Phase = v1alpha1.PhaseAccepted
	kc.Status.Ready = false
	err := sdk.Update(kc)
	if err != nil {
		logrus.Errorf("error initialising resource: %v", err)
		return err
	}
	return nil
}

func (h *Handler) finalizeKeycloak(kc *v1alpha1.Keycloak) error {
	logrus.Infof("finalise keycloak: %v", kc.Name)
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
