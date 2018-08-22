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
	"github.com/aerogear/keycloak-operator/pkg/apis/aerogear/v1alpha1"
)

const (
	KEYCLOAK_AUTH_URL = "auth/realms/master/protocol/openid-connect/token"
)

type Requester interface {
	Do(req *http.Request) (*http.Response, error)
}

type Client struct {
	requester Requester
	URL       string
	token     string
}

// generic type for keycloak spec resources
type T interface{}

//================================================= CREATE =================================================
func (c *Client) create(obj T, resourcePath, resourceName string) error {
	jsonValue, err := json.Marshal(obj)
	if err != nil {
		logrus.Errorf("error %+v marshalling object", err)
		return nil
	}

	logrus.Debugf("creating %s, %v, %v", resourceName, obj, string(jsonValue))

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

	logrus.Debugf("response status: %v, %v", res.StatusCode, res.Status)
	if res.StatusCode != 201 {
		return errors.New(fmt.Sprintf("failed to create %s: (%d) %s", resourceName, res.StatusCode, res.Status))
	}

	logrus.Debugf("response:", res)
	return nil
}

func (c *Client) CreateRealm(realm *v1alpha1.KeycloakRealm) error {
	err := c.create(realm, "realms", "realm")
	return err
}

func (c *Client) CreateClient(client *v1alpha1.KeycloakClient, realmName string) error {
	err := c.create(client, fmt.Sprintf("realms/%s/clients", realmName), "client")
	return err
}

func (c *Client) CreateUser(user *v1alpha1.KeycloakUser, realmName string) error {
	err := c.create(user, fmt.Sprintf("realms/%s/users", realmName), "user")
	return err
}

func (c *Client) CreateIdentityProvider(identityProvider *v1alpha1.KeycloakIdentityProvider, realmName string) error {
	err := c.create(identityProvider, fmt.Sprintf("realms/%s/identity-provider/instances", realmName), "identity provider")
	return err
}

//================================================= READ =================================================
func (c *Client) get(resourcePath, resourceName string, unMarshalFunc func(body []byte) (T, error)) (T, error) {
	req, err := http.NewRequest(
		"GET",
		fmt.Sprintf("%s/auth/admin/%s", c.URL, resourcePath),
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

	logrus.Debugf("response status: %v, %v", res.StatusCode, res.Status)
	if res.StatusCode != 200 {
		return nil, errors.New(fmt.Sprintf("failed to GET %s: (%d) %s", resourceName, res.StatusCode, res.Status))
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		logrus.Errorf("error reading response %+v", err)
		return nil, errors.Wrapf(err, "error reading %s GET response", resourceName)
	}

	logrus.Debugf("%s GET: %+v\n", resourceName, string(body))

	obj, err := unMarshalFunc(body)
	if err != nil {
		logrus.Error(err)
	}
	logrus.Debugf("%s GET= %#v", resourceName, obj)

	return obj, nil
}

func (c *Client) GetRealm(realmName string) (*v1alpha1.KeycloakRealm, error) {
	result, err := c.get(fmt.Sprintf("realms/%s", realmName), "realm", func(body []byte) (T, error) {
		var realm *v1alpha1.KeycloakRealm
		err := json.Unmarshal(body, realm)
		return realm, err
	})
	return result.(*v1alpha1.KeycloakRealm), err
}

func (c *Client) GetClient(clientID, realmName string) (*v1alpha1.KeycloakClient, error) {
	result, err := c.get(fmt.Sprintf("realms/%s/clients/%s", realmName, clientID), "client", func(body []byte) (T, error) {
		var client *v1alpha1.KeycloakClient
		err := json.Unmarshal(body, client)
		return client, err
	})
	return result.(*v1alpha1.KeycloakClient), err
}

func (c *Client) GetUser(userID, realmName string) (*v1alpha1.KeycloakUser, error) {
	result, err := c.get(fmt.Sprintf("realms/%s/users/%s", realmName, userID), "user", func(body []byte) (T, error) {
		var user *v1alpha1.KeycloakUser
		err := json.Unmarshal(body, user)
		return user, err
	})
	return result.(*v1alpha1.KeycloakUser), err
}

func (c *Client) GetIdentityProvider(alias string, realmName string) (*v1alpha1.KeycloakIdentityProvider, error) {
	result, err := c.get(fmt.Sprintf("realms/%s/identity-provider/instances/%s", realmName, alias), "identity provider", func(body []byte) (T, error) {
		var user *v1alpha1.KeycloakIdentityProvider
		err := json.Unmarshal(body, user)
		return user, err
	})
	return result.(*v1alpha1.KeycloakIdentityProvider), err
}

//================================================= UPDATE =================================================
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

	if res.StatusCode < 200 || res.StatusCode > 299 {
		logrus.Errorf("failed to UPDATE %s %v", resourceName, res.Status)
		return errors.New(fmt.Sprintf("failed to UPDATE %s: (%d) %s", resourceName, res.StatusCode, res.Status))
	}

	logrus.Debugf("response:", res)
	return nil
}

func (c *Client) UpdateRealm(specRealm *v1alpha1.KeycloakRealm) error {
	err := c.update(specRealm, fmt.Sprintf("realms/%s", specRealm.ID), "realm")
	return err
}

func (c *Client) UpdateClient(specClient *v1alpha1.KeycloakClient, realmName string) error {
	err := c.update(specClient, fmt.Sprintf("realms/%s/clients/%s", realmName, specClient.ID), "client")
	return err
}

func (c *Client) UpdateUser(specUser *v1alpha1.KeycloakUser, realmName string) error {
	err := c.update(specUser, fmt.Sprintf("realms/%s/users/%s", realmName, specUser.ID), "user")
	return err
}

func (c *Client) UpdateIdentityProvider(specIdentityProvider *v1alpha1.KeycloakIdentityProvider, realmName string) error {
	err := c.update(specIdentityProvider, fmt.Sprintf("realms/%s/identity-provider/instances/%s", realmName, specIdentityProvider.Alias), "identity provider")
	return err
}

//================================================= DELETE =================================================
func (c *Client) delete(resourcePath, resourceName string) error {
	req, err := http.NewRequest(
		"DELETE",
		fmt.Sprintf("%s/auth/admin/%s", c.URL, resourcePath),
		nil,
	)
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

	logrus.Debugf("response status: %v, %v", res.StatusCode, res.Status)
	if res.StatusCode != 204 {
		return errors.New(fmt.Sprintf("failed to DELETE %s: (%d) %s", resourceName, res.StatusCode, res.Status))
	}

	return nil
}

func (c *Client) DeleteRealm(realmName string) error {
	err := c.delete(fmt.Sprintf("realms/%s", realmName), "realm")
	return err
}

func (c *Client) DeleteClient(clientID, realmName string) error {
	err := c.delete(fmt.Sprintf("realms/%s/clients/%s", realmName, clientID), "client")
	return err
}

func (c *Client) DeleteUser(userID, realmName string) error {
	err := c.delete(fmt.Sprintf("realms/%s/users/%s", realmName, userID), "user")
	return err
}

func (c *Client) DeleteIdentityProvider(alias string, realmName string) error {
	err := c.delete(fmt.Sprintf("realms/%s/identity-provider/instances/%s", realmName, alias), "identity provider")
	return err
}

//================================================= LIST =================================================
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

	logrus.Debugf("response status: %v, %v", res.StatusCode, res.Status)
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return nil, errors.New(fmt.Sprintf("failed to LIST %s: (%d) %s", resourceName, res.StatusCode, res.Status))
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		logrus.Errorf("error reading response %+v", err)
		return nil, errors.Wrapf(err, "error reading %s LIST response", resourceName)
	}

	logrus.Debugf("%s LIST: %+v\n", resourceName, string(body))

	objs, err := unMarshalListFunc(body)
	if err != nil {
		logrus.Error(err)
	}
	logrus.Debugf("%s LIST= %#v", resourceName, objs)

	return objs, nil
}

func (c *Client) ListRealms() ([]*v1alpha1.KeycloakRealm, error) {
	result, err := c.list("realms", "realm", func(body []byte) (T, error) {
		var realms []*v1alpha1.KeycloakRealm
		err := json.Unmarshal(body, &realms)
		return realms, err
	})
	return result.([]*v1alpha1.KeycloakRealm), err
}

func (c *Client) ListClients(realmName string) ([]*v1alpha1.KeycloakClient, error) {
	result, err := c.list(fmt.Sprintf("realms/%s/clients", realmName), "clients", func(body []byte) (T, error) {
		var clients []*v1alpha1.KeycloakClient
		err := json.Unmarshal(body, &clients)
		return clients, err
	})
	return result.([]*v1alpha1.KeycloakClient), err
}

func (c *Client) ListUsers(realmName string) ([]*v1alpha1.KeycloakUser, error) {
	result, err := c.list(fmt.Sprintf("realms/%s/users", realmName), "users", func(body []byte) (T, error) {
		var users []*v1alpha1.KeycloakUser
		err := json.Unmarshal(body, &users)
		return users, err
	})
	return result.([]*v1alpha1.KeycloakUser), err
}

func (c *Client) ListIdentityProviders(realmName string) ([]*v1alpha1.KeycloakIdentityProvider, error) {
	result, err := c.list(fmt.Sprintf("realms/%s/identity-provider/instances", realmName), "identity providers", func(body []byte) (T, error) {
		var users []*v1alpha1.KeycloakIdentityProvider
		err := json.Unmarshal(body, &users)
		return users, err
	})
	return result.([]*v1alpha1.KeycloakIdentityProvider), err
}

//================================================= LOGIN =================================================
func (c *Client) login(user, pass string) error {
	form := url.Values{}
	form.Add("username", user)
	form.Add("password", pass)
	form.Add("client_id", "admin-cli")
	form.Add("grant_type", "password")

	req, err := http.NewRequest(
		"POST",
		fmt.Sprintf("%s/%s", c.URL, KEYCLOAK_AUTH_URL),
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

func defaultRequester() Requester {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	c := &http.Client{Transport: transport, Timeout: time.Second * 10}
	return c
}

type KeycloakInterface interface {
	CreateRealm(realm *v1alpha1.KeycloakRealm) error
	GetRealm(realmName string) (*v1alpha1.KeycloakRealm, error)
	UpdateRealm(specRealm *v1alpha1.KeycloakRealm) error
	DeleteRealm(realmName string) error
	ListRealms() ([]*v1alpha1.KeycloakRealm, error)

	CreateClient(client *v1alpha1.KeycloakClient, realmName string) error
	GetClient(clientID, realmName string) (*v1alpha1.KeycloakClient, error)
	UpdateClient(specClient *v1alpha1.KeycloakClient, realmName string) error
	DeleteClient(clientID, realmName string) error
	ListClients(realmName string) ([]*v1alpha1.KeycloakClient, error)

	CreateUser(user *v1alpha1.KeycloakUser, realmName string) error
	GetUser(userID, realmName string) (*v1alpha1.KeycloakUser, error)
	UpdateUser(specUser *v1alpha1.KeycloakUser, realmName string) error
	DeleteUser(userID, realmName string) error
	ListUsers(realmName string) ([]*v1alpha1.KeycloakUser, error)

	CreateIdentityProvider(identityProvider *v1alpha1.KeycloakIdentityProvider, realmName string) error
	GetIdentityProvider(alias, realmName string) (*v1alpha1.KeycloakIdentityProvider, error)
	UpdateIdentityProvider(specIdentityProvider *v1alpha1.KeycloakIdentityProvider, realmName string) error
	DeleteIdentityProvider(alias, realmName string) error
	ListIdentityProviders(realmName string) ([]*v1alpha1.KeycloakIdentityProvider, error)
}

type KeycloakClientFactory interface {
	AuthenticatedClient(kc v1alpha1.Keycloak, user, pass, url string) (KeycloakInterface, error)
}

type KeycloakFactory struct {
}

func (kf *KeycloakFactory) AuthenticatedClient(kc v1alpha1.Keycloak, user, pass, url string) (KeycloakInterface, error) {
	client := &Client{
		URL:       url,
		requester: defaultRequester(),
	}
	client.login(user, pass)

	return client, nil
}
