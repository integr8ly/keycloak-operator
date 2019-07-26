package keycloak

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"time"

	"fmt"

	"github.com/integr8ly/keycloak-operator/pkg/apis/aerogear/v1alpha1"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/typed/core/v1"
)

const (
	authUrl = "auth/realms/master/protocol/openid-connect/token"
)

type Requester interface {
	Do(req *http.Request) (*http.Response, error)
}

type Client struct {
	requester Requester
	URL       string
	token     string
}

// T is a generic type for keycloak spec resources
type T interface{}

// Generic create function for creating new Keycloak resources
func (c *Client) create(obj T, resourcePath, resourceName string) error {
	jsonValue, err := json.Marshal(obj)
	if err != nil {
		logrus.Errorf("error %+v marshalling object", err)
		return nil
	}

	req, err := http.NewRequest(
		"POST",
		fmt.Sprintf("%s/auth/admin/%s", c.URL, resourcePath),
		bytes.NewBuffer(jsonValue),
	)
	if err != nil {
		logrus.Errorf("error creating POST %s request %+v", resourceName, err)
		return errors.Wrapf(err, "error creating POST %s request", resourceName)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.token))
	res, err := c.requester.Do(req)

	if err != nil {
		logrus.Errorf("error on request %+v", err)
		return errors.Wrapf(err, "error performing POST %s request", resourceName)
	}
	defer res.Body.Close()

	if res.StatusCode != 201 && res.StatusCode != 204 {
		return fmt.Errorf("failed to create %s: (%d) %s", resourceName, res.StatusCode, res.Status)
	}

	if resourceName == "client" {
		d, _ := ioutil.ReadAll(res.Body)
		fmt.Println("user response ", string(d))
	}

	return nil
}

func (c *Client) CreateRealm(realm *v1alpha1.KeycloakRealm) error {
	apiRealm := realm.Spec.KeycloakApiRealm
	return c.create(apiRealm, "realms", "realm")
}

func (c *Client) CreateClient(client *v1alpha1.KeycloakClient, realmName string) error {
	return c.create(client.KeycloakApiClient, fmt.Sprintf("realms/%s/clients", realmName), "client")
}

func (c *Client) CreateUser(user *v1alpha1.KeycloakUser, realmName string) error {
	return c.create(user.KeycloakApiUser, fmt.Sprintf("realms/%s/users", realmName), "user")
}

func (c *Client) CreateUserClientRole(role *v1alpha1.KeycloakUserClientRole, realmName, clientID, userId string) error {
	return c.create(
		[]*v1alpha1.KeycloakUserClientRole{role},
		fmt.Sprintf("realms/%s/users/%s/role-mappings/clients/%s", realmName, userId, clientID),
		"user-client-role",
	)
}

func (c *Client) DeleteUserClientRole(role *v1alpha1.KeycloakUserClientRole, realmName, clientID, userId string) error {
	err := c.delete(
		fmt.Sprintf("realms/%s/users/%s/role-mappings/clients/%s", realmName, userId, clientID),
		"user-client-role",
		[]*v1alpha1.KeycloakUserClientRole{role},
	)
	return err
}

func (c *Client) UpdatePassword(user *v1alpha1.KeycloakApiUser, realmName, newPass string) error {
	passReset := &v1alpha1.KeycloakApiPasswordReset{}
	passReset.Type = "password"
	passReset.Temporary = false
	passReset.Value = newPass
	u := fmt.Sprintf("realms/%s/users/%s/reset-password", realmName, user.ID)
	if err := c.update(passReset, u, "paswordreset"); err != nil {
		return errors.Wrap(err, "error calling keycloak api ")
	}
	return nil
}

func (c *Client) FindUserByEmail(email, realm string) (*v1alpha1.KeycloakApiUser, error) {
	result, err := c.get(fmt.Sprintf("realms/%s/users?first=0&max=1&search=%s", realm, email), "user", func(body []byte) (T, error) {
		var users []*v1alpha1.KeycloakApiUser
		if err := json.Unmarshal(body, &users); err != nil {
			return nil, err
		}
		if len(users) == 0 {
			return nil, nil
		}
		return users[0], nil
	})
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, err
	}
	return result.(*v1alpha1.KeycloakApiUser), nil
}

func (c *Client) FindUserByUsername(name, realm string) (*v1alpha1.KeycloakApiUser, error) {
	result, err := c.get(fmt.Sprintf("realms/%s/users?first=0&max=1&search=%s", realm, name), "user", func(body []byte) (T, error) {
		var users []*v1alpha1.KeycloakApiUser
		if err := json.Unmarshal(body, &users); err != nil {
			return nil, err
		}
		if len(users) == 0 {
			return nil, errors.New("not found")
		}
		return users[0], nil
	})
	if err != nil {
		return nil, err
	}
	return result.(*v1alpha1.KeycloakApiUser), nil
}

func (c *Client) CreateIdentityProvider(identityProvider *v1alpha1.KeycloakIdentityProvider, realmName string) error {
	err := c.create(identityProvider, fmt.Sprintf("realms/%s/identity-provider/instances", realmName), "identity provider")
	return err
}

// Generic get function for returning a Keycloak resource
func (c *Client) get(resourcePath, resourceName string, unMarshalFunc func(body []byte) (T, error)) (T, error) {
	u := fmt.Sprintf("%s/auth/admin/%s", c.URL, resourcePath)
	req, err := http.NewRequest(
		"GET",
		u,
		nil,
	)
	if err != nil {
		logrus.Errorf("error creating GET %s request %+v", resourceName, err)
		return nil, errors.Wrapf(err, "error creating GET %s request", resourceName)
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.token))
	res, err := c.requester.Do(req)
	if err != nil {
		logrus.Errorf("error on request %+v", err)
		return nil, errors.Wrapf(err, "error performing GET %s request", resourceName)
	}

	if res.StatusCode != 200 {
		return nil, fmt.Errorf("failed to GET %s: (%d) %s", resourceName, res.StatusCode, res.Status)
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		logrus.Errorf("error reading response %+v", err)
		return nil, errors.Wrapf(err, "error reading %s GET response", resourceName)
	}

	obj, err := unMarshalFunc(body)
	if err != nil {
		logrus.Error(err)
	}
	return obj, nil
}

func (c *Client) GetRealm(realmName string) (*v1alpha1.KeycloakRealm, error) {
	result, err := c.get(fmt.Sprintf("realms/%s", realmName), "realm", func(body []byte) (T, error) {
		realm := &v1alpha1.KeycloakApiRealm{}
		err := json.Unmarshal(body, realm)
		return realm, err
	})
	if result == nil {
		return nil, nil
	}
	ret := &v1alpha1.KeycloakRealm{
		Spec: v1alpha1.KeycloakRealmSpec{
			KeycloakApiRealm: result.(*v1alpha1.KeycloakApiRealm),
		},
	}
	return ret, err
}

func (c *Client) GetClient(clientID, realmName string) (*v1alpha1.KeycloakClient, error) {
	result, err := c.get(fmt.Sprintf("realms/%s/clients/%s", realmName, clientID), "client", func(body []byte) (T, error) {
		client := &v1alpha1.KeycloakApiClient{}
		err := json.Unmarshal(body, client)
		return client, err
	})
	if err != nil {
		return nil, err
	}
	ret := &v1alpha1.KeycloakClient{
		KeycloakApiClient: result.(*v1alpha1.KeycloakApiClient),
	}
	return ret, err
}

func (c *Client) GetClientSecret(clientId, realmName string) (string, error) {
	//"https://{{ rhsso_route }}/auth/admin/realms/{{ rhsso_realm }}/clients/{{ rhsso_client_id }}/client-secret"
	result, err := c.get(fmt.Sprintf("realms/%s/clients/%s/client-secret", realmName, clientId), "client-secret", func(body []byte) (T, error) {
		res := map[string]string{}
		if err := json.Unmarshal(body, &res); err != nil {
			return nil, err
		}
		return res["value"], nil
	})
	if err != nil {
		return "", errors.Wrap(err, "failed to get: "+fmt.Sprintf("realms/%s/clients/%s/client-secret", realmName, clientId))
	}
	return result.(string), nil
}

func (c *Client) GetClientInstall(clientId, realmName string) ([]byte, error) {
	var response []byte
	if _, err := c.get(fmt.Sprintf("realms/%s/clients/%s/installation/providers/keycloak-oidc-keycloak-json", realmName, clientId), "client-installation", func(body []byte) (T, error) {
		response = body
		return body, nil
	}); err != nil {
		return nil, err
	}
	return response, nil
}

func (c *Client) GetUser(userID, realmName string) (*v1alpha1.KeycloakUser, error) {
	result, err := c.get(fmt.Sprintf("realms/%s/users/%s", realmName, userID), "user", func(body []byte) (T, error) {
		user := &v1alpha1.KeycloakApiUser{}
		err := json.Unmarshal(body, user)
		return user, err
	})
	if err != nil {
		return nil, err
	}
	ret := &v1alpha1.KeycloakUser{
		KeycloakApiUser: result.(*v1alpha1.KeycloakApiUser),
	}
	return ret, err
}

func (c *Client) GetIdentityProvider(alias string, realmName string) (*v1alpha1.KeycloakIdentityProvider, error) {
	result, err := c.get(fmt.Sprintf("realms/%s/identity-provider/instances/%s", realmName, alias), "identity provider", func(body []byte) (T, error) {
		provider := &v1alpha1.KeycloakIdentityProvider{}
		err := json.Unmarshal(body, provider)
		return provider, err
	})
	if err != nil {
		return nil, err
	}
	return result.(*v1alpha1.KeycloakIdentityProvider), err
}

// Generic put function for updating Keycloak resources
func (c *Client) update(obj T, resourcePath, resourceName string) error {
	jsonValue, err := json.Marshal(obj)
	if err != nil {
		return nil
	}

	req, err := http.NewRequest(
		"PUT",
		fmt.Sprintf("%s/auth/admin/%s", c.URL, resourcePath),
		bytes.NewBuffer(jsonValue),
	)
	if err != nil {
		logrus.Errorf("error creating UPDATE %s request %+v", resourceName, err)
		return errors.Wrapf(err, "error creating UPDATE %s request", resourceName)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Add("Authorization", "Bearer "+c.token)
	res, err := c.requester.Do(req)
	if err != nil {
		logrus.Errorf("error on request %+v", err)
		return errors.Wrapf(err, "error performing UPDATE %s request", resourceName)
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		logrus.Errorf("failed to UPDATE %s %v", resourceName, res.Status)
		return fmt.Errorf("failed to UPDATE %s: (%d) %s", resourceName, res.StatusCode, res.Status)
	}

	return nil
}

func (c *Client) UpdateRealm(realm *v1alpha1.KeycloakRealm) error {
	return c.update(realm, fmt.Sprintf("realms/%s", realm.Spec.ID), "realm")
}

func (c *Client) UpdateClient(specClient *v1alpha1.KeycloakClient, realmName string) error {
	return c.update(specClient.KeycloakApiClient, fmt.Sprintf("realms/%s/clients/%s", realmName, specClient.ID), "client")
}

func (c *Client) UpdateUser(specUser *v1alpha1.KeycloakUser, realmName string) error {
	return c.update(specUser.KeycloakApiUser, fmt.Sprintf("realms/%s/users/%s", realmName, specUser.ID), "user")
}

func (c *Client) UpdateIdentityProvider(specIdentityProvider *v1alpha1.KeycloakIdentityProvider, realmName string) error {
	return c.update(specIdentityProvider, fmt.Sprintf("realms/%s/identity-provider/instances/%s", realmName, specIdentityProvider.Alias), "identity provider")
}

// Generic delete function for deleting Keycloak resources
func (c *Client) delete(resourcePath, resourceName string, obj T) error {
	req, err := http.NewRequest(
		"DELETE",
		fmt.Sprintf("%s/auth/admin/%s", c.URL, resourcePath),
		nil,
	)

	if obj != nil {
		jsonValue, err := json.Marshal(obj)
		if err != nil {
			return nil
		}
		req, err = http.NewRequest(
			"DELETE",
			fmt.Sprintf("%s/auth/admin/%s", c.URL, resourcePath),
			bytes.NewBuffer(jsonValue),
		)
		req.Header.Set("Content-Type", "application/json")
	}

	if err != nil {
		logrus.Errorf("error creating DELETE %s request %+v", resourceName, err)
		return errors.Wrapf(err, "error creating DELETE %s request", resourceName)
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.token))
	res, err := c.requester.Do(req)
	if err != nil {
		logrus.Errorf("error on request %+v", err)
		return errors.Wrapf(err, "error performing DELETE %s request", resourceName)
	}
	defer res.Body.Close()
	if res.StatusCode != 204 {
		return fmt.Errorf("failed to DELETE %s: (%d) %s", resourceName, res.StatusCode, res.Status)
	}

	return nil
}

func (c *Client) DeleteRealm(realmName string) error {
	err := c.delete(fmt.Sprintf("realms/%s", realmName), "realm", nil)
	return err
}

func (c *Client) DeleteClient(clientID, realmName string) error {
	err := c.delete(fmt.Sprintf("realms/%s/clients/%s", realmName, clientID), "client", nil)
	return err
}

func (c *Client) DeleteUser(userID, realmName string) error {
	err := c.delete(fmt.Sprintf("realms/%s/users/%s", realmName, userID), "user", nil)
	return err
}

func (c *Client) DeleteIdentityProvider(alias string, realmName string) error {
	err := c.delete(fmt.Sprintf("realms/%s/identity-provider/instances/%s", realmName, alias), "identity provider", nil)
	return err
}

// Generic list function for listing Keycloak resources
func (c *Client) list(resourcePath, resourceName string, unMarshalListFunc func(body []byte) (T, error)) (T, error) {
	req, err := http.NewRequest(
		"GET",
		fmt.Sprintf("%s/auth/admin/%s", c.URL, resourcePath),
		nil,
	)
	if err != nil {
		logrus.Errorf("error creating LIST %s request %+v", resourceName, err)
		return nil, errors.Wrapf(err, "error creating LIST %s request", resourceName)
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.token))
	res, err := c.requester.Do(req)
	if err != nil {
		logrus.Errorf("error on request %+v", err)
		return nil, errors.Wrapf(err, "error performing LIST %s request", resourceName)
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode > 299 {
		return nil, fmt.Errorf("failed to LIST %s: (%d) %s", resourceName, res.StatusCode, res.Status)
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		logrus.Errorf("error reading response %+v", err)
		return nil, errors.Wrapf(err, "error reading %s LIST response", resourceName)
	}

	objs, err := unMarshalListFunc(body)
	if err != nil {
		logrus.Error(err)
	}

	return objs, nil
}

func (c *Client) ListRealms() ([]*v1alpha1.KeycloakRealm, error) {
	result, err := c.list("realms", "realm", func(body []byte) (T, error) {
		var realms []*v1alpha1.KeycloakRealm
		err := json.Unmarshal(body, &realms)
		return realms, err
	})
	resultAsRealm, ok := result.([]*v1alpha1.KeycloakRealm)
	if !ok {
		return nil, err
	}
	return resultAsRealm, err
}

func (c *Client) ListClients(realmName string) ([]*v1alpha1.KeycloakClient, error) {
	result, err := c.list(fmt.Sprintf("realms/%s/clients", realmName), "clients", func(body []byte) (T, error) {
		var clients []*v1alpha1.KeycloakClient
		err := json.Unmarshal(body, &clients)
		return clients, err
	})

	if err != nil {
		return nil, err
	}

	res, ok := result.([]*v1alpha1.KeycloakClient)

	if !ok {
		return nil, errors.New("error decoding list clients response")
	}

	return res, nil

}

func (c *Client) ListUsers(realmName string) ([]*v1alpha1.KeycloakUser, error) {
	result, err := c.list(fmt.Sprintf("realms/%s/users", realmName), "users", func(body []byte) (T, error) {
		var users []*v1alpha1.KeycloakUser
		err := json.Unmarshal(body, &users)
		return users, err
	})
	if err != nil {
		return nil, err
	}
	return result.([]*v1alpha1.KeycloakUser), err
}

func (c *Client) ListIdentityProviders(realmName string) ([]*v1alpha1.KeycloakIdentityProvider, error) {
	result, err := c.list(fmt.Sprintf("realms/%s/identity-provider/instances", realmName), "identity providers", func(body []byte) (T, error) {
		var providers []*v1alpha1.KeycloakIdentityProvider
		err := json.Unmarshal(body, &providers)
		return providers, err
	})
	if err != nil {
		return nil, err
	}
	return result.([]*v1alpha1.KeycloakIdentityProvider), err
}

func (c *Client) ListUserClientRoles(realmName, clientID, userID string) ([]*v1alpha1.KeycloakUserClientRole, error) {
	objects, err := c.list("realms/"+realmName+"/users/"+userID+"/role-mappings/clients/"+clientID, "userClientRoles", func(body []byte) (t T, e error) {
		var userClientRoles []*v1alpha1.KeycloakUserClientRole
		err := json.Unmarshal(body, &userClientRoles)
		return userClientRoles, err
	})
	if err != nil {
		return nil, err
	}
	return objects.([]*v1alpha1.KeycloakUserClientRole), err
}

func (c *Client) ListAvailableUserClientRoles(realmName, clientID, userID string) ([]*v1alpha1.KeycloakUserClientRole, error) {
	objects, err := c.list("realms/"+realmName+"/users/"+userID+"/role-mappings/clients/"+clientID+"/available", "userClientRoles", func(body []byte) (t T, e error) {
		var userClientRoles []*v1alpha1.KeycloakUserClientRole
		err := json.Unmarshal(body, &userClientRoles)
		return userClientRoles, err
	})
	if err != nil {
		return nil, err
	}
	return objects.([]*v1alpha1.KeycloakUserClientRole), err
}

func (c *Client) Ping() error {
	u := c.URL + "/auth/"
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		logrus.Errorf("error creating ping request %+v", err)
		return errors.Wrap(err, "error creating ping request")
	}

	res, err := c.requester.Do(req)
	if err != nil {
		logrus.Errorf("error on request %+v", err)
		return errors.Wrapf(err, "error performing ping request")
	}

	logrus.Debugf("response status: %v, %v", res.StatusCode, res.Status)
	if res.StatusCode != 200 {
		return fmt.Errorf("failed to ping, response status code: %v", res.StatusCode)
	}
	defer res.Body.Close()

	return nil
}

// login requests a new auth token from Keycloak
func (c *Client) login(user, pass string) error {
	form := url.Values{}
	form.Add("username", user)
	form.Add("password", pass)
	form.Add("client_id", "admin-cli")
	form.Add("grant_type", "password")

	req, err := http.NewRequest(
		"POST",
		fmt.Sprintf("%s/%s", c.URL, authUrl),
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		return errors.Wrap(err, "error creating login request")
	}

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	res, err := c.requester.Do(req)
	if err != nil {
		logrus.Errorf("error on request %+v", err)
		return errors.Wrap(err, "error performing token request")
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		logrus.Errorf("error reading response %+v", err)
		return errors.Wrap(err, "error reading token response")
	}

	tokenRes := &v1alpha1.TokenResponse{}
	err = json.Unmarshal(body, tokenRes)
	if err != nil {
		return errors.Wrap(err, "error parsing token response")
	}

	if tokenRes.Error != "" {
		logrus.Errorf("error with request: " + tokenRes.ErrorDescription)
		return errors.New(tokenRes.ErrorDescription)
	}

	c.token = tokenRes.AccessToken

	return nil
}

// defaultRequester returns a default client for requesting http endpoints
func defaultRequester() Requester {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	c := &http.Client{Transport: transport, Timeout: time.Second * 10}
	return c
}

//go:generate moq -out keycloakClient_moq.go . KeycloakInterface

type KeycloakInterface interface {
	Ping() error

	CreateRealm(realm *v1alpha1.KeycloakRealm) error
	GetRealm(realmName string) (*v1alpha1.KeycloakRealm, error)
	UpdateRealm(specRealm *v1alpha1.KeycloakRealm) error
	DeleteRealm(realmName string) error
	ListRealms() ([]*v1alpha1.KeycloakRealm, error)

	CreateClient(client *v1alpha1.KeycloakClient, realmName string) error
	GetClient(clientID, realmName string) (*v1alpha1.KeycloakClient, error)
	GetClientSecret(clientId, realmName string) (string, error)
	GetClientInstall(clientId, realmName string) ([]byte, error)
	UpdateClient(specClient *v1alpha1.KeycloakClient, realmName string) error
	DeleteClient(clientID, realmName string) error
	ListClients(realmName string) ([]*v1alpha1.KeycloakClient, error)

	CreateUser(user *v1alpha1.KeycloakUser, realmName string) error
	UpdatePassword(user *v1alpha1.KeycloakApiUser, realmName, newPass string) error
	FindUserByEmail(email, realm string) (*v1alpha1.KeycloakApiUser, error)
	FindUserByUsername(name, realm string) (*v1alpha1.KeycloakApiUser, error)
	GetUser(userID, realmName string) (*v1alpha1.KeycloakUser, error)
	UpdateUser(specUser *v1alpha1.KeycloakUser, realmName string) error
	DeleteUser(userID, realmName string) error
	ListUsers(realmName string) ([]*v1alpha1.KeycloakUser, error)

	CreateIdentityProvider(identityProvider *v1alpha1.KeycloakIdentityProvider, realmName string) error
	GetIdentityProvider(alias, realmName string) (*v1alpha1.KeycloakIdentityProvider, error)
	UpdateIdentityProvider(specIdentityProvider *v1alpha1.KeycloakIdentityProvider, realmName string) error
	DeleteIdentityProvider(alias, realmName string) error
	ListIdentityProviders(realmName string) ([]*v1alpha1.KeycloakIdentityProvider, error)

	CreateUserClientRole(role *v1alpha1.KeycloakUserClientRole, realmName, clientID, userId string) error
	ListUserClientRoles(realmName, clientID, userID string) ([]*v1alpha1.KeycloakUserClientRole, error)
	ListAvailableUserClientRoles(realmName, clientID, userID string) ([]*v1alpha1.KeycloakUserClientRole, error)
	DeleteUserClientRole(role *v1alpha1.KeycloakUserClientRole, realmName, clientID, userID string) error
}

//go:generate moq -out keycloakClientFactory_moq.go . KeycloakClientFactory

//KeycloakClientFactory interface
type KeycloakClientFactory interface {
	AuthenticatedClient(kc v1alpha1.Keycloak) (KeycloakInterface, error)
}

type KeycloakFactory struct {
	SecretClient v1.SecretInterface
}

// AuthenticatedClient returns an authenticated client for requesting endpoints from the Keycloak api
func (kf *KeycloakFactory) AuthenticatedClient(kc v1alpha1.Keycloak) (KeycloakInterface, error) {
	adminCreds, err := kf.SecretClient.Get(kc.Spec.AdminCredentials, v12.GetOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the admin credentials")
	}
	user := string(adminCreds.Data["SSO_ADMIN_USERNAME"])
	pass := string(adminCreds.Data["SSO_ADMIN_PASSWORD"])
	url := string(adminCreds.Data["SSO_ADMIN_URL"])
	client := &Client{
		URL:       url,
		requester: defaultRequester(),
	}
	if err := client.login(user, pass); err != nil {
		return nil, err
	}
	return client, nil
}
