package keycloak

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"time"

	"crypto/tls"

	"github.com/aerogear/keycloak-operator/pkg/apis/aerogear/v1alpha1"
)

var (
	keycloakAuthURL = "auth/realms/master/protocol/openid-connect/token"
)

func NewKeycloakResourceClient() {

}

type requester interface {
	Do(req *http.Request) (*http.Response, error)
}

type Client struct {
	Requester requester
	URL       string
	token     string
}

type TokenResponse struct {
	AccessToken      string `json:"access_token"`
	ExpiresIn        int    `json:"expires_in"`
	RefreshExpiresIn int    `json:"refresh_expires_in"`
	RefreshToken     string `json:"refresh_token"`
	TokenType        string `json:"token_type"`
	NotBeforePolicy  int    `json:"not-before-policy"`
	SessionState     string `json:"session_state"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

func (c *Client) ListRealms() (map[string]*v1alpha1.KeycloakRealm, error) {

	req, err := http.NewRequest(
		"GET",
		c.URL+"/admin/realms",
		nil,
	)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", "Bearer "+c.token)
	res, err := c.Requester.Do(req)
	if err != nil {
		logrus.Infof("error on request %+v", err)
		return nil, errors.Wrap(err, "error performing realms list request")
	}
	logrus.Infof(c.token)
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return nil, errors.New("failed to list realms: " + " (" + strconv.Itoa(res.StatusCode) + ") " + res.Status)
	}
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		logrus.Infof("error reading response %+v", err)
		return nil, errors.Wrap(err, "error reading realms list response")
	}

	logrus.Infof("realms list: %+v\n", string(body))
	// err = json.Unmarshal(body, tokenRes)
	// if err != nil {
	// 	return nil, errors.Wrap(err, "error parsing realms list response")
	// }

	return map[string]*v1alpha1.KeycloakRealm{}, nil
}

func (c *Client) GetRealm(name string) (*v1alpha1.KeycloakRealm, error) {
	return &v1alpha1.KeycloakRealm{}, nil
}

func (c *Client) UpdateRealm(realm *v1alpha1.KeycloakRealm) error {
	return nil
}

func (c *Client) DeleteRealm(name string) error {
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
	res, err := c.Requester.Do(req)

	if err != nil {
		logrus.Infof("error on request %+v", err)
		return errors.Wrap(err, "error performing token request")
	}
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		logrus.Infof("error reading response %+v", err)
		return errors.Wrap(err, "error reading token response")
	}
	tokenRes := &TokenResponse{}

	err = json.Unmarshal(body, tokenRes)
	if err != nil {
		return errors.Wrap(err, "error parsing token response")
	}

	if tokenRes.Error != "" {
		logrus.Infof("error with request: " + tokenRes.ErrorDescription)
		return errors.New(tokenRes.ErrorDescription)
	}

	c.token = tokenRes.AccessToken

	return nil
}

func defaultRequester() requester {
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	c := &http.Client{Timeout: time.Second * 10}
	return c
}

type KeycloakInterface interface {
	ListRealms() (map[string]*v1alpha1.KeycloakRealm, error)
	GetRealm(realmName string) (*v1alpha1.KeycloakRealm, error)
	UpdateRealm(realm *v1alpha1.KeycloakRealm) error
	DeleteRealm(realmName string) error
}

type KeycloakClientFactory interface {
	AuthenticatedClient(kc v1alpha1.Keycloak, user, pass, url string) (KeycloakInterface, error)
}

type KeycloakFactory struct {
}

func (kf *KeycloakFactory) AuthenticatedClient(kc v1alpha1.Keycloak, user, pass, url string) (KeycloakInterface, error) {
	client := &Client{
		URL:       url,
		Requester: defaultRequester(),
	}
	logrus.Infof("going to login")
	client.login(user, pass)

	return client, nil
}
