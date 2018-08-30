package keycloak

import (
	"context"
	"fmt"

	"reflect"

	"strings"

	"encoding/json"

	"github.com/aerogear/keycloak-operator/pkg/apis/aerogear/v1alpha1"
	"github.com/google/uuid"
	sc "github.com/kubernetes-incubator/service-catalog/pkg/api/meta"
	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1beta1"
	scclientset "github.com/kubernetes-incubator/service-catalog/pkg/client/clientset_generated/clientset"
	apps "github.com/openshift/client-go/apps/clientset/versioned/typed/apps/v1"
	routev1 "github.com/openshift/client-go/route/clientset/versioned/typed/route/v1"
	"github.com/operator-framework/operator-sdk/pkg/sdk"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	SSO_TEMPLATE_NAME     = "sso72-x509-postgresql-persistent.json"
	SSO_ROUTE_NAME        = "sso"
	SSO_SECURE_ROUTE_NAME = "secure-sso"
	SSO_APPLICATION_NAME  = "sso"
	SSO_TEMPLATE_PATH     = "./deploy/template"
)

type K8sCruder interface {
	Create(sdk.Object) error
}

type Handler struct {
	cfg                  v1alpha1.Config
	k8sClient            kubernetes.Interface
	kcClientFactory      KeycloakClientFactory
	kubeconfig           *rest.Config
	serviceCatalogClient scclientset.Interface
	defaultClients       map[string]struct{}
	k8sCrud              K8sCruder
}

type ServiceClassExternalMetadata struct {
	ServiceName string `json:"serviceName"`
}

func NewHandler(cfg v1alpha1.Config, kcClientFactory KeycloakClientFactory, svcCatalog scclientset.Interface, k8sClient kubernetes.Interface, cruder K8sCruder) *Handler {
	kcDefaultClients := []string{"account", "admin-cli", "broker", "realm-management", "security-admin-console"}
	set := make(map[string]struct{}, len(kcDefaultClients))
	for _, s := range kcDefaultClients {
		set[s] = struct{}{}
	}

	kubeconfig, _ := getKubeconfig()
	return &Handler{
		cfg:                  cfg,
		kcClientFactory:      kcClientFactory,
		serviceCatalogClient: svcCatalog,
		k8sClient:            k8sClient,
		defaultClients:       set,
		kubeconfig:           kubeconfig,
		k8sCrud:              cruder,
	}
}

func (h *Handler) accepted(keycloak *v1alpha1.Keycloak) (*v1alpha1.Keycloak, error) {
	//make a copy of state and modify it then return it
	kc := keycloak.DeepCopy()
	if kc.Spec.AdminCredentials != "" {
		return nil, nil
	}

	adminPwd, err := h.generatePassword()
	if err != nil {
		return nil, err
	}
	namespace := keycloak.ObjectMeta.Namespace
	adminCredRef, err := h.createAdminCredentials(namespace, "admin", adminPwd)
	if err != nil {
		return nil, err
	}

	kc.Spec.AdminCredentials = adminCredRef.GetName()
	kc.Status.Phase = v1alpha1.PhaseReadyForProvision
	return kc, nil

}

	case v1alpha1.PhaseCredentialsCreated:
		adminCreds, err := h.k8sClient.CoreV1().Secrets(namespace).Get(kc.Spec.AdminCredentials, metav1.GetOptions{})
		if err != nil {
			return errors.Wrap(err, "failed to get the secret for the admin credentials")
		}
		decodedParams := map[string]string{}
		for k, v := range adminCreds.Data {
			decodedParams[k] = string(v)
		}
		err = install(kcCopy, decodedParams)
		if err != nil {
			return errors.Wrap(err, "failed to install sso")
		}
		return sdk.Update(kcState)

		kcCopy.Status.Phase = v1alpha1.PhaseWaitForPodsToRun
		return sdk.Update(kcCopy)

	case v1alpha1.PhaseWaitForPodsToRun:
		ready, err := h.areSsoPodReady(namespace)
		if err != nil {
			return err
		}
		if ready {
			kcCopy.Status.Phase = v1alpha1.PhaseProvisioned
			kcCopy.Status.Ready = true
		}
	case v1alpha1.PhaseProvisioned:
		routeClient, _ := routev1.NewForConfig(h.kubeconfig)
		routeList, _ := routeClient.Routes(namespace).List(metav1.ListOptions{})
		for _, route := range routeList.Items {
			if route.Spec.To.Name == SSO_ROUTE_NAME {
				url := fmt.Sprintf("http://%v", route.Spec.Host)
				h.addEntryToSecret(namespace, kc.Spec.AdminCredentials, "SSO_ADMIN_URL", url)
			}
			if route.Spec.To.Name == SSO_SECURE_ROUTE_NAME {
				url := fmt.Sprintf("https://%v", route.Spec.Host)
				h.addEntryToSecret(namespace, kc.Spec.AdminCredentials, "SSO_SECURE_ADMIN_URL", url)
			}
		}
		kcCopy.Status.Phase = v1alpha1.PhaseComplete

	case v1alpha1.PhaseComplete:
		err := h.reconcileResources(kcCopy)
		if err != nil {
			kc.Status.Phase = v1alpha1.PhaseFailed
			kc.Status.Message = err.Error()
			logrus.Error("failed to reconcile resources ", err)
			return sdk.Update(kc)
		}
		return nil
		//if err := sdk.Get(kc, sdk.WithGetOptions(&metav1.GetOptions{})); err != nil {
		//	return nil
		//}
		//// set back to provisioned and we will reconcile again next time through
		//kc.Status.Phase = v1alpha1.PhaseProvisioned
		//return sdk.Update(kc)

	case v1alpha1.PhaseDeprovisioning:
		err := h.deleteKeycloak(kcCopy)
		if err != nil {
			return errors.Wrap(err, "failed to deprovision")
		}
		//todo what about finalizer if it keeps failing
		kcCopy.Status.Phase = v1alpha1.PhaseDeprovisioned
		return h.finalizeKeycloak(kcCopy)

	}
	return nil
}
func (h *Handler) areSsoPodReady(namespace string) (bool, error) {

	podList, err := h.k8sClient.CoreV1().Pods(namespace).List(metav1.ListOptions{
		LabelSelector:        fmt.Sprintf("application=%v", SSO_APPLICATION_NAME),
		IncludeUninitialized: false,
	})

	if err != nil || len(podList.Items) == 0 {
		return false, err
	}
	fmt.Println("found sso pods", len(podList.Items))
	for _, pod := range podList.Items {
		for _, condition := range pod.Status.Conditions {
			if condition.Type == "Ready" && condition.Status != "True" {
				return false, nil
			}
		}
	}
	return true, nil
}

func getKubeconfig() (*rest.Config, error) {
	kubeconfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{},
	)
	cfg, err := kubeconfig.ClientConfig()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create kubeconfig")
	}
	return cfg, nil
}

func (h *Handler) reconcileResources(kc *v1alpha1.Keycloak) error {
	logrus.Infof("reconcile resources (%v)", kc.Name)
	adminCreds, err := h.k8sClient.CoreV1().Secrets(kc.Namespace).Get(kc.Spec.AdminCredentials, metav1.GetOptions{})
	if err != nil {
		return errors.Wrap(err, "failed to get the admin credentials")
	}
	user := string(adminCreds.Data["SSO_ADMIN_USERNAME"])
	pass := string(adminCreds.Data["SSO_ADMIN_PASSWORD"])
	url := string(adminCreds.Data["SSO_SECURE_ADMIN_URL"])
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

func (h *Handler) createAdminCredentials(namespace, username, password string) (*corev1.Secret, error) {
	adminCredentialsSecret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Labels:       map[string]string{"application": "sso"},
			Namespace:    namespace,
			GenerateName: "keycloak-admin-cred-",
		},
		StringData: map[string]string{
			"SSO_ADMIN_USERNAME": username,
			"SSO_ADMIN_PASSWORD": password,
		},
		Type: "Opaque",
	}

	if err := h.k8sCrud.Create(adminCredentialsSecret); err != nil {
		return nil, errors.Wrap(err, "failed to create secret for the admin credentials")
	}

	return adminCredentialsSecret, nil
}
func (h *Handler) addEntryToSecret(namespace, secretName, key, value string) error {
	secret, err := h.k8sClient.CoreV1().Secrets(namespace).Get(secretName, metav1.GetOptions{})

	secret.StringData = map[string]string{}
	secret.StringData[key] = value
	_, err = h.k8sClient.CoreV1().Secrets(namespace).Update(secret)
	if err != nil {
		return errors.Wrap(err, "could not update admin credentials")
	}
	return nil
}

func (h *Handler) generatePassword() (string, error) {
	generatedPassword, err := uuid.NewRandom()
	if err != nil {
		return "", errors.Wrap(err, "error generating password")
	}
	return strings.Replace(generatedPassword.String(), "-", "", 10), nil
}

func (h *Handler) deleteKeycloak(kc *v1alpha1.Keycloak) error {
	namespace := kc.ObjectMeta.Namespace
	deleteOpts := metav1.NewDeleteOptions(0)
	listOpts := metav1.ListOptions{LabelSelector: "application=sso"}
	dc, err := apps.NewForConfig(h.kubeconfig)
	if err != nil {
		return err
	}
	// delete dcs
	if err := dc.DeploymentConfigs(namespace).DeleteCollection(deleteOpts, listOpts); err != nil {
		return errors.Wrap(err, "failed to remove the deployment configs")
	}
	// delete pvc
	if err := h.k8sClient.CoreV1().PersistentVolumeClaims(kc.Namespace).DeleteCollection(deleteOpts, listOpts); err != nil {
		return errors.Wrap(err, "failed to remove the pvc")
	}
	// delete routes
	routeClient, err := routev1.NewForConfig(h.kubeconfig)
	if err := routeClient.Routes(kc.Namespace).DeleteCollection(deleteOpts, listOpts); err != nil {
		return errors.Wrap(err, "failed to remove the routes")
	}

	// delete secrets
	if err := h.k8sClient.CoreV1().Secrets(kc.Namespace).DeleteCollection(deleteOpts, listOpts); err != nil {
		return errors.Wrap(err, "failed to remove the secrets")
	}
	// delete services
	services, err := h.k8sClient.CoreV1().Services(kc.Namespace).List(listOpts)
	if err != nil {
		return errors.Wrap(err, "failed to list all services for sso")
	}
	// todo handle more than one error
	for _, s := range services.Items {
		err = h.k8sClient.CoreV1().Services(kc.Namespace).Delete(s.Name, deleteOpts)
	}
	if err != nil {
		return errors.Wrap(err, "error deleteing service")
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

func (h *Handler) reconcileRealm(kcRealm, specRealm *v1alpha1.KeycloakRealm, kcClient KeycloakInterface) error {
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

func (h *Handler) reconcileClients(kcClient KeycloakInterface, specRealm *v1alpha1.KeycloakRealm) error {
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

func (h *Handler) init(keycloak *v1alpha1.Keycloak) (*v1alpha1.Keycloak, error) {
	logrus.Infof("initialise keycloak: %v", keycloak.Name)
	// copy and return new state
	kc := keycloak.DeepCopy()
	sc.AddFinalizer(kc, v1alpha1.KeycloakFinalizer)
	kc.Status.Phase = v1alpha1.PhaseAccepted
	kc.Status.Ready = false
	return kc, nil
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
