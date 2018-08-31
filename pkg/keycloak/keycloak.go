package keycloak

import (
	"context"

	"reflect"

	"strings"

	"github.com/aerogear/keycloak-operator/pkg/apis/aerogear/v1alpha1"
	"github.com/google/uuid"
	apps "github.com/openshift/client-go/apps/clientset/versioned/typed/apps/v1"
	routev1 "github.com/openshift/client-go/route/clientset/versioned/typed/route/v1"
	"github.com/operator-framework/operator-sdk/pkg/k8sclient"
	"github.com/operator-framework/operator-sdk/pkg/sdk"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	SSO_TEMPLATE_NAME    = "sso72-x509-postgresql-persistent.json"
	SSO_ROUTE_NAME       = "sso"
	SSO_APPLICATION_NAME = "sso"
	SSO_TEMPLATE_PATH    = "./deploy/template"
)

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

func (h *Reconciler) Handle(ctx context.Context, event sdk.Event) error {
	logrus.Debug("handling object ", event.Object.GetObjectKind().GroupVersionKind().String())

	kc := event.Object.(*v1alpha1.Keycloak)
	kcCopy := kc.DeepCopy()
	logrus.Infof("Keycloak: %v, Phase: %v", kc.Name, kc.Status.Phase)

	if event.Deleted {
		kcState, err := h.phaseHandler.Deprovision(kcCopy)
		if err != nil {
			return errors.Wrap(err, "failed to deprovision")
		}
		return sdk.Update(kcState)
	}
	switch kc.Status.Phase {
	case v1alpha1.NoPhase:
		kcState, err := h.phaseHandler.Initialise(kc)
		if err != nil {
			return errors.Wrap(err, "failed to init resource")
		}
		return sdk.Update(kcState)

	case v1alpha1.PhaseAccepted:
		kcState, err := h.phaseHandler.Accepted(kc)
		if err != nil {
			return errors.Wrap(err, "phase accepted failed")
		}
		return sdk.Update(kcState)
	case v1alpha1.PhaseProvision:

		kcState, err := h.phaseHandler.Provision(kc)
		if err != nil {
			return errors.Wrap(err, "phase provision failed")
		}
		return sdk.Update(kcState)
	case v1alpha1.PhaseProvisioned:
		kcState, err := h.phaseHandler.Provisioned(kc)
		if err != nil {
			return errors.Wrap(err, "phase provisioned failed")
		}
		return sdk.Update(kcState)
	case v1alpha1.PhaseComplete:
		err := h.reconcileResources(kcCopy)
		if err != nil {
			return errors.Wrap(err, "failed to reconcile resources ")
		}
		return nil
	}
	return nil
}

func (h *Reconciler) reconcileResources(kc *v1alpha1.Keycloak) error {
	logrus.Infof("reconcile resources (%v)", kc.Name)
	kcClient, err := h.kcClientFactory.AuthenticatedClient(*kc)
	if err != nil {
		return errors.Wrap(err, "failed to get authenticated client for keycloak")
	}
	err = h.reconcileRealms(kc, kcClient)
	if err != nil {
		return errors.Wrap(err, "error reconciling realms")
	}
	return nil
}

func (h *Reconciler) reconcileRealms(kc *v1alpha1.Keycloak, kcClient KeycloakInterface) error {
	logrus.Infof("reconcile realms (%v)", kc.Name)

	specRealms := kc.Spec.Realms
	kcRealms, err := kcClient.ListRealms()
	if err != nil {
		return errors.Wrap(err, "error retrieving realms from keycloak")
	}

	kcRealmMap := map[string]*v1alpha1.KeycloakRealm{}
	for i := range kcRealms {
		logrus.Debugf("kc realm %v", kcRealms[i].ID)
		kcRealmMap[kcRealms[i].Realm] = kcRealms[i]
	}

	for i := range specRealms {
		logrus.Debugf("spec realm %v", specRealms[i].ID)
		err = h.reconcileRealm(kcRealmMap[specRealms[i].Realm], &specRealms[i], kcClient)
		//This should try and reconcile all realms rather throwing an error on the first failure
		if err != nil {
			return errors.Wrapf(err, "error reconciling realm: %v", specRealms[i].ID)
		}
	}

	return nil
}

func (h *Reconciler) reconcileRealm(kcRealm, specRealm *v1alpha1.KeycloakRealm, kcClient KeycloakInterface) error {
	logrus.Infof("reconcile realm (%v)", specRealm.Realm)

	if kcRealm == nil {
		logrus.Debugf("create realm: %v", specRealm.Realm)
		err := kcClient.CreateRealm(specRealm)
		if err != nil {
			logrus.Errorf("error creating realm %v : %v", specRealm.ID, err)
			return errors.Wrap(err, "error creating keycloak realm")
		}
	} else {
		if h.cfg.SyncResources {
			logrus.Debugf("sync realm %v", specRealm.Realm)
			if !resourcesEqual(kcRealm, specRealm) {
				err := kcClient.UpdateRealm(specRealm)
				if err != nil {
					logrus.Errorf("error updating realm %v", specRealm.Realm)
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

func (h *Reconciler) reconcileClients(kcClient KeycloakInterface, specRealm *v1alpha1.KeycloakRealm) error {
	logrus.Infof("reconcile clients (%v)", specRealm.Realm)

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

func (h *Reconciler) reconcileClient(kcClient, specClient *v1alpha1.KeycloakClient, realmName string, authenticatedClient KeycloakInterface) error {
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

func (h *Reconciler) isDefaultClient(client string) bool {
	_, ok := h.defaultClients[client]
	return ok
}

func (h *Reconciler) reconcileUsers(kcClient KeycloakInterface, specRealm *v1alpha1.KeycloakRealm) error {
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

func (h *Reconciler) reconcileUser(kcUser, specUser *v1alpha1.KeycloakUser, realmName string, authenticatedClient KeycloakInterface) error {
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

func (h *Reconciler) reconcileIdentityProviders(kcClient KeycloakInterface, specRealm *v1alpha1.KeycloakRealm) error {
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

func (h *Reconciler) reconcileIdentityProvider(kcIdentityProvider, specIdentityProvider *v1alpha1.KeycloakIdentityProvider, realmName string, authenticatedClient KeycloakInterface) error {
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
