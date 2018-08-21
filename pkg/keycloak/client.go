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
	"strconv"

	"github.com/aerogear/keycloak-operator/pkg/apis/aerogear/v1alpha1"
)

var (
	keycloakAuthURL = "auth/realms/master/protocol/openid-connect/token"
)

func NewKeycloakResourceClient() {

}

type Requester interface {
	Do(req *http.Request) (*http.Response, error)
}

type Client struct {
	requester Requester
	URL       string
	token     string
}

func (c *Client) ListRealms() ([]*v1alpha1.KeycloakRealm, error) {
	req, err := http.NewRequest(
		"GET",
		fmt.Sprintf("%s/auth/admin/realms", c.URL),
		nil,
	)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Authorization", "Bearer "+c.token)
	res, err := c.requester.Do(req)
	if err != nil {
		logrus.Errorf("error on request %+v", err)
		return nil, errors.Wrap(err, "error performing realms list request")
	}

	if res.StatusCode < 200 || res.StatusCode > 299 {
		return nil, errors.New("failed to list realms: " + " (" + strconv.Itoa(res.StatusCode) + ") " + res.Status)
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		logrus.Errorf("error reading response %+v", err)
		return nil, errors.Wrap(err, "error reading realms list response")
	}

	var realms []*v1alpha1.KeycloakRealm
	err = json.Unmarshal(body, &realms)

	if err != nil {
		logrus.Error(err)
	}

	return realms, err
}

func (c *Client) CreateRealm(realm *v1alpha1.KeycloakRealm) error {
	jsonValue, err := json.Marshal(realm)
	if err != nil {
		return nil
	}

	logrus.Debugf("creating realm, %v, %v", realm, string(jsonValue))

	req, err := http.NewRequest(
		"POST",
		fmt.Sprintf("%s/auth/admin/realms/", c.URL),
		bytes.NewBuffer(jsonValue),
	)
	if err != nil {
		logrus.Errorf("error creating request %+v", err)
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Add("Authorization", "Bearer "+c.token)
	res, err := c.requester.Do(req)

	if err != nil {
		logrus.Errorf("error on request %+v", err)
		return err
	}

	logrus.Debugf("response status: %v, %v", res.StatusCode, res.Status)
	if res.StatusCode != 201 {
		return errors.New("failed to create realm: " + " (" + strconv.Itoa(res.StatusCode) + ") " + res.Status)
	}

	logrus.Debugf("response:", res)

	return nil
}

func (c *Client) GetRealm(name string) (*v1alpha1.KeycloakRealm, error) {
	req, err := http.NewRequest(
		"GET",
		fmt.Sprintf("%s/auth/admin/realms/%s", c.URL, name),
		nil,
	)
	if err != nil {
		logrus.Errorf("error creating request %+v", err)
		return nil, err
	}

	req.Header.Add("Authorization", "Bearer "+c.token)
	res, err := c.requester.Do(req)
	if err != nil {
		logrus.Errorf("error on request %+v", err)
		return nil, err
	}

	logrus.Debugf("response status: %v, %v", res.StatusCode, res.Status)
	if res.StatusCode != 200 {
		return nil, errors.New("failed to get realm: " + " (" + strconv.Itoa(res.StatusCode) + ") " + res.Status)
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		logrus.Errorf("error reading response %+v", err)
		return nil, errors.Wrap(err, "error reading realms list response")
	}

	logrus.Debugf("realm: %+v\n", string(body))

	var realm *v1alpha1.KeycloakRealm
	err = json.Unmarshal(body, &realm)

	if err != nil {
		logrus.Error(err)
	}
	logrus.Debugf("realm = %#v", realm)

	return realm, nil
}

func (c *Client) UpdateRealm(realm *v1alpha1.KeycloakRealm) error {
	jsonValue, err := json.Marshal(realm)
	if err != nil {
		return nil
	}

	req, err := http.NewRequest(
		"PUT",
		fmt.Sprintf("%s/auth/admin/realms/%s", c.URL, realm.ID),
		bytes.NewBuffer(jsonValue),
	)
	if err != nil {
		logrus.Errorf("error creating request %+v", err)
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Add("Authorization", "Bearer "+c.token)
	res, err := c.requester.Do(req)
	if err != nil {
		logrus.Errorf("error on request %+v", err)
		return err
	}

	if res.StatusCode < 200 || res.StatusCode > 299 {
		logrus.Errorf("failed to update realm %v", res.Status)
		return errors.New("failed to update realm: " + " (" + strconv.Itoa(res.StatusCode) + ") " + res.Status)
	}

	return nil
}

func (c *Client) login(user, pass string) error {
	form := url.Values{}
	form.Add("username", user)
	form.Add("password", pass)
	form.Add("client_id", "admin-cli")
	form.Add("grant_type", "password")

	req, err := http.NewRequest(
		"POST",
		c.URL+"/auth/realms/master/protocol/openid-connect/token",
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

func (c *Client) GetClient(clientID string, realmName string) (*v1alpha1.KeycloakClient, error) {
	req, err := http.NewRequest(
		"GET",
		fmt.Sprintf("%s/auth/admin/realms/%s/clients/%s", c.URL, realmName, clientID),
		nil,
	)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Authorization", "Bearer "+c.token)
	res, err := c.requester.Do(req)
	if err != nil {
		logrus.Errorf("error on request %+v", err)
		return nil, errors.Wrap(err, "error performing create client request")
	}

	if res.StatusCode != 200 {
		logrus.Errorf("error reading response %+v", err)
		return nil, errors.New("failed to get client: " + " (" + strconv.Itoa(res.StatusCode) + ") " + res.Status)
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		logrus.Errorf("error reading response %+v", err)
		return nil, errors.Wrap(err, "error reading get client response")
	}

	var client *v1alpha1.KeycloakClient
	err = json.Unmarshal(body, &client)
	if err != nil {
		logrus.Errorf("failed to unmarshal client %+v", err)
		return nil, errors.Wrap(err, "failed to unmarshal client")
	}

	return client, nil
}

func (c *Client) CreateClient(client *v1alpha1.KeycloakClient, realmName string) error {
	body, err := json.Marshal(client)
	if err != nil {
		return nil
	}

	req, err := http.NewRequest(
		"POST",
		fmt.Sprintf("%s/auth/admin/realms/%s/clients", c.URL, realmName),
		bytes.NewBuffer(body),
	)
	if err != nil {
		return err
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "Bearer "+c.token)
	res, err := c.requester.Do(req)
	if err != nil {
		logrus.Errorf("error on request %+v", err)
		return errors.Wrap(err, "error performing create client request")
	}

	if res.StatusCode < 201 || res.StatusCode > 409 {
		return errors.New("failed to create client: " + " (" + strconv.Itoa(res.StatusCode) + ") " + res.Status)
	}

	return nil
}

func (c *Client) DeleteClient(clientID string, realmName string) error {
	req, err := http.NewRequest(
		"DELETE",
		fmt.Sprintf("%s/auth/admin/realms/%s/clients/%s", c.URL, realmName, clientID),
		nil,
	)
	if err != nil {
		return err
	}

	req.Header.Add("Authorization", "Bearer "+c.token)
	res, err := c.requester.Do(req)
	if err != nil {
		logrus.Errorf("error on request %+v", err)
		return errors.Wrap(err, "error performing delete client request")
	}

	if res.StatusCode != 204 {
		return errors.New("failed to delete client: " + " (" + strconv.Itoa(res.StatusCode) + ") " + res.Status)
	}

	return nil
}

func (c *Client) UpdateClient(specClient *v1alpha1.KeycloakClient, realmName string) error {
	body, err := json.Marshal(specClient)
	if err != nil {
		return nil
	}

	req, err := http.NewRequest(
		"PUT",
		fmt.Sprintf("%s/auth/admin/realms/%s/clients/%s", c.URL, realmName, specClient.ID),
		bytes.NewBuffer(body),
	)
	if err != nil {
		return err
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "Bearer "+c.token)
	res, err := c.requester.Do(req)
	if err != nil {
		logrus.Errorf("error on request %+v", err)
		return errors.Wrap(err, "error performing create client request")
	}

	if res.StatusCode != 204 {
		return errors.New("failed to update client: " + " (" + strconv.Itoa(res.StatusCode) + ") " + res.Status)
	}

	return nil
}

func (c *Client) ListClients(realmName string) ([]*v1alpha1.KeycloakClient, error) {
	req, err := http.NewRequest(
		"GET",
		fmt.Sprintf("%s/auth/admin/realms/%s/clients", c.URL, realmName),
		nil,
	)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Authorization", "Bearer "+c.token)
	res, err := c.requester.Do(req)
	if err != nil {
		logrus.Errorf("error on request %+v", err)
		return nil, errors.Wrap(err, "error performing clients list request")
	}

	if res.StatusCode < 200 || res.StatusCode > 299 {
		return nil, errors.New("failed to list clients: " + " (" + strconv.Itoa(res.StatusCode) + ") " + res.Status)
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		logrus.Errorf("error reading response %+v", err)
		return nil, errors.Wrap(err, "error reading clients list response")
	}

	clients := []*v1alpha1.KeycloakClient{}
	if err := json.Unmarshal(body, &clients); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal clients list")
	}

	return clients, nil
}

func (c *Client) CreateUser(user *v1alpha1.KeycloakUser, realmName string) error {
	body, err := json.Marshal(user)
	if err != nil {
		return nil
	}

	req, err := http.NewRequest(
		"POST",
		fmt.Sprintf("%s/auth/admin/realms/%s/users", c.URL, realmName),
		bytes.NewBuffer(body),
	)
	if err != nil {
		return err
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "Bearer "+c.token)
	res, err := c.requester.Do(req)
	if err != nil {
		logrus.Errorf("error on request %+v", err)
		return errors.Wrap(err, "error performing create user request")
	}

	if res.StatusCode < 201 || res.StatusCode > 409 {
		return errors.New("failed to create user: " + " (" + strconv.Itoa(res.StatusCode) + ") " + res.Status)
	}

	return nil
}

func (c *Client) DeleteUser(userID string, realmName string) error {
	req, err := http.NewRequest(
		"DELETE",
		fmt.Sprintf("%s/auth/admin/realms/%s/users/%s", c.URL, realmName, userID),
		nil,
	)
	if err != nil {
		return err
	}

	req.Header.Add("Authorization", "Bearer "+c.token)
	res, err := c.requester.Do(req)
	if err != nil {
		logrus.Errorf("error on request %+v", err)
		return errors.Wrap(err, "error performing delete user request")
	}

	if res.StatusCode != 204 {
		return errors.New("failed to delete user: " + " (" + strconv.Itoa(res.StatusCode) + ") " + res.Status)
	}

	return nil
}

func (c *Client) UpdateUser(specUser *v1alpha1.KeycloakUser, realmName string) error {
	body, err := json.Marshal(specUser)
	if err != nil {
		return nil
	}

	req, err := http.NewRequest(
		"PUT",
		fmt.Sprintf("%s/auth/admin/realms/%s/users/%s", c.URL, realmName, specUser.ID),
		bytes.NewBuffer(body),
	)
	if err != nil {
		return err
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "Bearer "+c.token)
	res, err := c.requester.Do(req)
	if err != nil {
		logrus.Errorf("error on request %+v", err)
		return errors.Wrap(err, "error performing create user request")
	}

	if res.StatusCode != 204 {
		return errors.New("failed to update user: " + " (" + strconv.Itoa(res.StatusCode) + ") " + res.Status)
	}

	return nil
}

func (c *Client) ListUsers(realmName string) ([]*v1alpha1.KeycloakUser, error) {
	req, err := http.NewRequest(
		"GET",
		fmt.Sprintf("%s/auth/admin/realms/%s/users", c.URL, realmName),
		nil,
	)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Authorization", "Bearer "+c.token)
	res, err := c.requester.Do(req)
	if err != nil {
		logrus.Errorf("error on request %+v", err)
		return nil, errors.Wrap(err, "error performing users list request")
	}

	if res.StatusCode < 200 || res.StatusCode > 299 {
		return nil, errors.New("failed to list users: " + " (" + strconv.Itoa(res.StatusCode) + ") " + res.Status)
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		logrus.Errorf("error reading response %+v", err)
		return nil, errors.Wrap(err, "error reading users list response")
	}

	users := []*v1alpha1.KeycloakUser{}
	if err := json.Unmarshal(body, &users); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal users list")
	}

	return users, nil
}

func (c *Client) CreateIdentityProvider(identityProvider *v1alpha1.KeycloakIdentityProvider, realmName string) error {
	body, err := json.Marshal(identityProvider)
	if err != nil {
		return nil
	}

	req, err := http.NewRequest(
		"POST",
		fmt.Sprintf("%s/auth/admin/realms/%s/identity-provider/instances", c.URL, realmName),
		bytes.NewBuffer(body),
	)
	if err != nil {
		return err
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "Bearer "+c.token)
	res, err := c.requester.Do(req)
	if err != nil {
		logrus.Errorf("error on request %+v", err)
		return errors.Wrap(err, "error performing create identity provider request")
	}

	if res.StatusCode != 201 {
		logrus.Errorf("failed to create identity provider %+v", res)
		return errors.New("failed to create identity provider: " + " (" + strconv.Itoa(res.StatusCode) + ") " + res.Status)
	}

	return nil
}

func (c *Client) GetIdentityProvider(alias string, realmName string) (*v1alpha1.KeycloakIdentityProvider, error) {
	req, err := http.NewRequest(
		"GET",
		fmt.Sprintf("%s/auth/admin/realms/%s/identity-provider/instances/%s", c.URL, realmName, alias),
		nil,
	)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Authorization", "Bearer "+c.token)
	res, err := c.requester.Do(req)
	if err != nil {
		logrus.Errorf("error on request %+v", err)
		return nil, errors.Wrap(err, "error performing create identity provider request")
	}

	if res.StatusCode != 200 {
		logrus.Errorf("error reading response %+v", err)
		return nil, errors.New("failed to get identity provider: " + " (" + strconv.Itoa(res.StatusCode) + ") " + res.Status)
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		logrus.Errorf("error reading response %+v", err)
		return nil, errors.Wrap(err, "error reading get identity provider response")
	}

	var identityProvider *v1alpha1.KeycloakIdentityProvider
	err = json.Unmarshal(body, &identityProvider)
	if err != nil {
		logrus.Errorf("failed to unmarshal identity provider %+v", err)
		return nil, errors.Wrap(err, "failed to unmarshal identity provider")
	}

	return identityProvider, nil
}

func (c *Client) UpdateIdentityProvider(identityProvider *v1alpha1.KeycloakIdentityProvider, realmName string) error {
	body, err := json.Marshal(identityProvider)
	if err != nil {
		return nil
	}

	req, err := http.NewRequest(
		"PUT",
		fmt.Sprintf("%s/auth/admin/realms/%s/identity-provider/instances/%s", c.URL, realmName, identityProvider.Alias),
		bytes.NewBuffer(body),
	)
	if err != nil {
		return err
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "Bearer "+c.token)
	res, err := c.requester.Do(req)

	if err != nil {
		logrus.Errorf("error on request %+v", err)
		return errors.Wrap(err, "error performing create identity provider request")
	}

	if res.StatusCode != 204 {
		logrus.Errorf("failed to update identity provider :  %+v", res)
		return errors.New("failed to update identity provider : " + " (" + strconv.Itoa(res.StatusCode) + ") " + res.Status)
	}

	return nil
}


func (c *Client) DeleteIdentityProvider(alias string, realmName string) error {
	req, err := http.NewRequest(
		"DELETE",
		fmt.Sprintf("%s/auth/admin/realms/%s/identity-provider/instances/%s", c.URL, realmName, alias),
		nil,
	)
	if err != nil {
		return err
	}

	req.Header.Add("Authorization", "Bearer "+c.token)
	res, err := c.requester.Do(req)
	if err != nil {
		logrus.Errorf("error on request %+v", err)
		return errors.Wrap(err, "error performing delete identity provider request")
	}

	if res.StatusCode != 204 {
		return errors.New("failed to delete identity provider: " + " (" + strconv.Itoa(res.StatusCode) + ") " + res.Status)
	}

	return nil
}

func (c *Client) ListIdentityProviders(realmName string) ([]*v1alpha1.KeycloakIdentityProvider, error) {
	req, err := http.NewRequest(
		"GET",
		fmt.Sprintf("%s/auth/admin/realms/%s/identity-provider/instances", c.URL, realmName),
		nil,
	)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Authorization", "Bearer "+c.token)
	res, err := c.requester.Do(req)
	if err != nil {
		logrus.Errorf("error on request %+v", err)
		return nil, errors.Wrap(err, "error performing identity providers list request")
	}

	if res.StatusCode < 200 || res.StatusCode > 299 {
		return nil, errors.New("failed to list identity providers: " + " (" + strconv.Itoa(res.StatusCode) + ") " + res.Status)
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		logrus.Errorf("error reading response %+v", err)
		return nil, errors.Wrap(err, "error reading identity providers list response")
	}

	identityProviders := []*v1alpha1.KeycloakIdentityProvider{}
	if err := json.Unmarshal(body, &identityProviders); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal identity providers list")
	}

	return identityProviders, nil
}


func defaultRequester() Requester {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	c := &http.Client{Transport: transport, Timeout: time.Second * 10}
	return c
}

type KeycloakInterface interface {
	ListRealms() ([]*v1alpha1.KeycloakRealm, error)
	GetRealm(realmName string) (*v1alpha1.KeycloakRealm, error)
	CreateRealm(realm *v1alpha1.KeycloakRealm) error
	UpdateRealm(realm *v1alpha1.KeycloakRealm) error

	GetClient(clientID, realmName string) (*v1alpha1.KeycloakClient, error)
	CreateClient(client *v1alpha1.KeycloakClient, realmName string) error
	DeleteClient(clientID, realmName string) error
	UpdateClient(specClient *v1alpha1.KeycloakClient, realmName string) error
	ListClients(realmName string) ([]*v1alpha1.KeycloakClient, error)

	CreateUser(user *v1alpha1.KeycloakUser, realmName string) error
	DeleteUser(userID, realmName string) error
	UpdateUser(specUser *v1alpha1.KeycloakUser, realmName string) error
	ListUsers(realmName string) ([]*v1alpha1.KeycloakUser, error)

	CreateIdentityProvider(identityProvider *v1alpha1.KeycloakIdentityProvider, realmName string) error
	GetIdentityProvider(alias, realmName string) (*v1alpha1.KeycloakIdentityProvider, error)
	UpdateIdentityProvider(identityProvider *v1alpha1.KeycloakIdentityProvider, realmName string) error
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
