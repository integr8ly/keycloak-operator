package realm

import (
	"github.com/sirupsen/logrus"
	"reflect"
	"strings"

	"github.com/integr8ly/keycloak-operator/pkg/apis/aerogear/v1alpha1"
	"github.com/integr8ly/keycloak-operator/pkg/keycloak"
	"github.com/integr8ly/keycloak-operator/pkg/util"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type phaseHandler struct {
	k8sClient       kubernetes.Interface
	sdk             keycloak.SdkCruder
	operatorNS      string
	kcClientFactory keycloak.KeycloakClientFactory
	defaultClients  map[string]struct{}
}

func NewPhaseHandler(k8sClient kubernetes.Interface, sdk keycloak.SdkCruder, operatorNS string, kcFactory keycloak.KeycloakClientFactory) *phaseHandler {
	kcDefaultClients := []string{"account", "admin-cli", "broker", "realm-management", "security-admin-console"}
	set := make(map[string]struct{}, len(kcDefaultClients))
	for _, s := range kcDefaultClients {
		set[s] = struct{}{}
	}
	return &phaseHandler{
		k8sClient:       k8sClient,
		sdk:             sdk,
		operatorNS:      operatorNS,
		kcClientFactory: kcFactory,
		defaultClients:  set,
	}
}
func (ph *phaseHandler) PreflightChecks(kcr *v1alpha1.KeycloakRealm) (*v1alpha1.KeycloakRealm, error) {
	if kcr.Status.KeycloakName == "" || kcr.Status.Phase == v1alpha1.PhaseInstanceDeprovisioned {
		// no preflight check required
		return kcr, nil
	}

	authClient, err := ph.getClient(kcr)
	if err != nil {
		return kcr, err
	}

	err = authClient.Ping()
	if err != nil {
		return kcr, errors.Wrap(err, "failed preflight checks")
	}
	return kcr, nil
}

func (ph *phaseHandler) Initialise(kcr *v1alpha1.KeycloakRealm) (*v1alpha1.KeycloakRealm, error) {
	if err := v1alpha1.AddFinalizer(kcr, v1alpha1.KeycloakFinalizer); err != nil {
		return nil, err
	}
	kcr.Status.Phase = v1alpha1.PhaseAccepted
	return kcr, nil
}

func (ph *phaseHandler) Accepted(kcr *v1alpha1.KeycloakRealm) (*v1alpha1.KeycloakRealm, error) {
	//look for a provisioned keycloak instance
	list := &v1alpha1.KeycloakList{
		TypeMeta: metav1.TypeMeta{
			Kind:       "keycloak",
			APIVersion: v1alpha1.Group + "/" + v1alpha1.Version,
		},
	}
	err := ph.sdk.List(ph.operatorNS, list)
	if err != nil {
		return nil, err
	}

	for _, kc := range list.Items {
		if kc.Status.Phase == v1alpha1.PhaseAwaitProvision {
			kc.Status.Phase = v1alpha1.PhaseProvision
			ph.sdk.Update(&kc)
		}
		if kc.Status.Phase == v1alpha1.PhaseComplete {
			kcr.Status.KeycloakName = kc.Name
			kcr.Status.Phase = v1alpha1.PhaseProvision
		}
	}
	return kcr, nil
}

func (ph *phaseHandler) Provision(kcr *v1alpha1.KeycloakRealm) (*v1alpha1.KeycloakRealm, error) {
	kcClient, err := ph.getClient(kcr)
	if err != nil {
		return kcr, err
	}

	realm, err := kcClient.GetRealm(kcr.Spec.Realm)
	if err != nil {
		return kcr, errors.Wrap(err, "error retrieving realms from keycloak")
	}

	if realm != nil && realm.Spec.Realm == kcr.Spec.Realm {
		kcr.Status.Phase = v1alpha1.PhaseReconcile
		return kcr, nil
	}

	storeUsers := kcr.Spec.KeycloakApiRealm.Users
	storeClients := kcr.Spec.KeycloakApiRealm.Clients

	kcr.Spec.KeycloakApiRealm.Clients = []*v1alpha1.KeycloakClient{}
	kcr.Spec.KeycloakApiRealm.Users = []*v1alpha1.KeycloakUser{}

	err = kcClient.CreateRealm(kcr)

	kcr.Spec.KeycloakApiRealm.Clients = storeClients
	kcr.Spec.KeycloakApiRealm.Users = storeUsers

	if err != nil {
		return kcr, errors.Wrap(err, "error creating keycloak realm")
	}

	kcr.Status.Phase = v1alpha1.PhaseReconcile

	return kcr, nil
}

func (ph *phaseHandler) Reconcile(kcr *v1alpha1.KeycloakRealm) (*v1alpha1.KeycloakRealm, error) {
	kcClient, err := ph.getClient(kcr)
	if err != nil {
		return kcr, errors.Wrapf(err, "error reconciling keycloak realm: '%v'", kcr.Spec.Realm)
	}

	errors := util.NewMultiError()
	errors.AppendMultiErrorer(ph.reconcileUsers(kcClient, kcr, kcr.ObjectMeta.Namespace))
	errors.AppendMultiErrorer(ph.reconcileClients(kcClient, kcr, kcr.ObjectMeta.Namespace))
	errors.AppendMultiErrorer(ph.reconcileIdentityProviders(kcClient, kcr))

	if !errors.IsNil() {
		return kcr, errors
	}

	return kcr, nil
}

func (ph *phaseHandler) reconcileUsers(kcClient keycloak.KeycloakInterface, realm *v1alpha1.KeycloakRealm, ns string) util.MultiErrorer {

	users, err := kcClient.ListUsers(realm.Spec.Realm)
	if err != nil {
		retErr := util.NewMultiError()
		retErr.AddError(err)
		return retErr
	}

	userPairsList := map[string]*v1alpha1.KeycloakUserPair{}

	for i := range users {
		userPairsList[users[i].UserName] = &v1alpha1.KeycloakUserPair{
			KcUser:   users[i],
			SpecUser: nil,
		}
	}

	for i := range realm.Spec.Users {
		user := realm.Spec.Users[i]
		if _, ok := userPairsList[user.UserName]; ok {
			userPairsList[user.UserName].SpecUser = user
		} else {
			userPairsList[user.UserName] = &v1alpha1.KeycloakUserPair{
				SpecUser: user,
				KcUser:   nil,
			}
		}
	}
	errors := util.NewMultiError()
	for i := range userPairsList {
		errors.AddError(ph.reconcileUser(userPairsList[i].KcUser, userPairsList[i].SpecUser, realm.Spec.Realm, kcClient, ns))
	}
	return errors
}

func (ph *phaseHandler) reconcileUser(kcUser, specUser *v1alpha1.KeycloakUser, realmName string, authenticatedClient keycloak.KeycloakInterface, ns string) error {
	if specUser == nil {
		return authenticatedClient.DeleteUser(kcUser.ID, realmName)
	}
	if kcUser == nil {
		err := authenticatedClient.CreateUser(specUser, realmName)
		if err != nil {
			return err
		}
		// generate and update password
		u, err := authenticatedClient.FindUserByEmail(specUser.UserName, realmName)
		if err != nil {
			return errors.Wrap(err, "failed find user")
		}
		if u == nil {
			return errors.New("failed to find user " + specUser.Email)
		}
		var newPass string
		if specUser.Password != nil {
			newPass = *specUser.Password
			specUser.Password = nil
		} else {
			newPass, err = keycloak.GeneratePassword()
			if err != nil {
				return err
			}
		}
		if err := authenticatedClient.UpdatePassword(u, realmName, newPass); err != nil {
			return errors.Wrap(err, "failed to update password for user "+u.Email)
		}
		data := map[string][]byte{"username": []byte(specUser.UserName), "password": []byte(newPass)}
		userSecret := corev1.Secret{
			TypeMeta: v1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Secret",
			},
			ObjectMeta: v1.ObjectMeta{
				Labels:    map[string]string{"application": "sso", "realm": realmName},
				Namespace: ns,
				Name:      *specUser.OutputSecret,
			},
			Data: data,
			Type: "Opaque",
		}
		if _, err := ph.k8sClient.CoreV1().Secrets(ns).Create(&userSecret); err != nil {
			return errors.Wrap(err, "failed to create secret ")
		}

	} else {
		if specUser.Password != nil {
			specUser.Password = nil
		}
		if !resourcesEqual(kcUser.KeycloakApiUser, specUser.KeycloakApiUser) {
			specUser.ID = kcUser.ID
			err := authenticatedClient.UpdateUser(specUser, realmName)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (ph *phaseHandler) reconcileClients(kcClient keycloak.KeycloakInterface, realm *v1alpha1.KeycloakRealm, ns string) util.MultiErrorer {
	clients, err := kcClient.ListClients(realm.Spec.Realm)
	if err != nil {
		retErr := util.NewMultiError()
		retErr.AddError(err)
		return retErr
	}

	clientPairsList := map[string]*v1alpha1.KeycloakClientPair{}

	for i := range clients {
		clientPairsList[clients[i].ClientID] = &v1alpha1.KeycloakClientPair{
			SpecClient: nil,
			KcClient:   clients[i],
		}
	}

	for i := range realm.Spec.Clients {
		client := realm.Spec.Clients[i]
		if _, ok := clientPairsList[client.ClientID]; ok {
			clientPairsList[client.ClientID].SpecClient = client
		} else {
			clientPairsList[client.ClientID] = &v1alpha1.KeycloakClientPair{
				KcClient:   nil,
				SpecClient: client,
			}
		}
	}
	errors := util.NewMultiError()
	for i := range clientPairsList {
		errors.AddError(ph.reconcileClient(clientPairsList[i].KcClient, clientPairsList[i].SpecClient, realm.Spec.Realm, kcClient, ns))
	}
	return errors
}

func (ph *phaseHandler) isDefaultClient(client string) bool {
	_, ok := ph.defaultClients[client]
	return ok
}

func (ph *phaseHandler) reconcileClient(kcClient, specClient *v1alpha1.KeycloakClient, realmName string, authenticatedClient keycloak.KeycloakInterface, ns string) error {
	if specClient == nil && !ph.isDefaultClient(kcClient.ClientID) {
		if err := authenticatedClient.DeleteClient(kcClient.ID, realmName); err != nil {
			return err
		}
	} else if kcClient == nil {
		if err := authenticatedClient.CreateClient(specClient, realmName); err != nil {
			return err
		}
	} else {
		if !resourcesEqual(kcClient, specClient) && !ph.isDefaultClient(kcClient.ClientID) {
			specClient.ID = kcClient.ID
			if err := authenticatedClient.UpdateClient(specClient, realmName); err != nil {
				return err
			}
		}
	}
	logrus.Info("reconciling client", specClient)
	if specClient != nil && specClient.OutputSecret != nil {
		cs, err := authenticatedClient.GetClientSecret(specClient.ID, realmName)
		if err != nil {
			return err
		}
		clientJSON, err := authenticatedClient.GetClientInstall(specClient.ID, realmName)
		if err != nil {
			return err
		}

		data := map[string][]byte{"secret": []byte(cs), "install": clientJSON}
		clientSecret := &corev1.Secret{
			TypeMeta: v1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Secret",
			},
			ObjectMeta: v1.ObjectMeta{
				Labels:    map[string]string{"application": "sso", "realm": realmName},
				Namespace: ns,
				Name:      *specClient.OutputSecret,
			},
			Data: data,
			Type: "Opaque",
		}
		if _, err := ph.k8sClient.CoreV1().Secrets(ns).Create(clientSecret); err != nil {
			if !strings.Contains(err.Error(), "already exists") {
				return errors.Wrap(err, "failed to create client secret")
			}
			if _, err := ph.k8sClient.CoreV1().Secrets(ns).Update(clientSecret); err != nil {
				return errors.Wrap(err, "failed to update client secret")
			}

			return nil
		}
	}

	return nil
}

func (ph *phaseHandler) reconcileIdentityProviders(kcClient keycloak.KeycloakInterface, realm *v1alpha1.KeycloakRealm) util.MultiErrorer {
	identityProviders, err := kcClient.ListIdentityProviders(realm.Spec.Realm)
	if err != nil {
		retErr := util.NewMultiError()
		retErr.AddError(err)
		return retErr
	}

	kcIdentityProviders := map[string]*v1alpha1.KeycloakIdentityProvider{}
	for i := range identityProviders {
		kcIdentityProviders[identityProviders[i].Alias] = identityProviders[i]
	}

	identityProviderPairsList := map[string]*v1alpha1.KeycloakIdentityProviderPair{}
	for i := range realm.Spec.IdentityProviders {
		identityProvider := realm.Spec.IdentityProviders[i]
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

	errors := util.NewMultiError()
	for i := range identityProviderPairsList {
		errors.AddError(ph.reconcileIdentityProvider(identityProviderPairsList[i].KcIdentityProvider, identityProviderPairsList[i].SpecIdentityProvider, realm.Spec.Realm, kcClient))
	}

	return errors
}

func (ph *phaseHandler) reconcileIdentityProvider(kcIdentityProvider, specIdentityProvider *v1alpha1.KeycloakIdentityProvider, realmName string, authenticatedClient keycloak.KeycloakInterface) error {
	if specIdentityProvider == nil {
		if err := authenticatedClient.DeleteIdentityProvider(kcIdentityProvider.Alias, realmName); err != nil {
			return err
		}
		return nil
	} else if kcIdentityProvider == nil {
		if err := authenticatedClient.CreateIdentityProvider(specIdentityProvider, realmName); err != nil {
			return err
		}
		return nil
	}
	//The API doesn't return the secret, so in order to stop in never being equal we just set it to the spec version
	kcIdentityProvider.Config["clientSecret"] = specIdentityProvider.Config["clientSecret"]
	//Ensure the internalID is set on the spec object, this is required for update requests to succeed
	specIdentityProvider.InternalID = kcIdentityProvider.InternalID
	if !resourcesEqual(kcIdentityProvider, specIdentityProvider) {
		err := authenticatedClient.UpdateIdentityProvider(specIdentityProvider, realmName)
		if err != nil {
			return err
		}
	}

	return nil
}

func (ph *phaseHandler) Deprovision(realm *v1alpha1.KeycloakRealm) (*v1alpha1.KeycloakRealm, error) {
	kcClient, err := ph.getClient(realm)
	if err != nil {
		return realm, err
	}

	//delete client secrets
	for _, client := range realm.Spec.Clients {
		if client.OutputSecret == nil {
			continue
		}
		clientSecret := &corev1.Secret{
			TypeMeta: v1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Secret",
			},
			ObjectMeta: v1.ObjectMeta{
				Namespace: realm.GetNamespace(),
				Name:      *client.OutputSecret,
			},
		}
		ph.sdk.Delete(clientSecret)
	}

	//delete user secrets
	for _, user := range realm.Spec.Users {
		userSecret := &corev1.Secret{
			TypeMeta: v1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Secret",
			},
			ObjectMeta: v1.ObjectMeta{
				Namespace: realm.GetNamespace(),
				Name:      *user.OutputSecret,
			},
		}
		ph.sdk.Delete(userSecret)
	}

	err = kcClient.DeleteRealm(realm.Spec.ID)
	if err != nil && !strings.Contains(err.Error(), "404") {
		return realm, err
	}

	return realm, nil
}

func (ph *phaseHandler) getClient(kcr *v1alpha1.KeycloakRealm) (keycloak.KeycloakInterface, error) {
	//look for a provisioned keycloak instance
	list := &v1alpha1.KeycloakList{
		TypeMeta: metav1.TypeMeta{
			Kind:       "keycloak",
			APIVersion: v1alpha1.Group + "/" + v1alpha1.Version,
		},
	}
	err := ph.sdk.List(ph.operatorNS, list)
	if err != nil {
		return nil, err
	}
	logrus.Infof("Got %v keycloaks", len(list.Items))
	for _, kc := range list.Items {
		logrus.Infof("'%v' ?? '%v'", kc.Name, kcr.Status.KeycloakName)
		if kc.Name == kcr.Status.KeycloakName {
			return ph.kcClientFactory.AuthenticatedClient(kc)
		}
	}

	return nil, errors.New("Could not find keycloak instance: " + kcr.Status.KeycloakName)
}

func resourcesEqual(obj1, obj2 keycloak.T) bool {
	return reflect.DeepEqual(obj1, obj2)
}
