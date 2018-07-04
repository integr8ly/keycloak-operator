package keycloak

import (
	"net/http"

	"time"

	"fmt"

	"net/url"

	"io/ioutil"

	"crypto/tls"

	"strings"

	"encoding/json"

	"github.com/aerogear/keycloak-operator/pkg/apis/areogear/v1alpha1"
	"github.com/pkg/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
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
	auth      auth
	Requester requester
}

func defaultRequester() requester {
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	c := &http.Client{Timeout: time.Second * 10}
	return c
}

func NewAuthenticatedClientForKeycloak(keycloak v1alpha1.Keycloak, k8sClient kubernetes.Interface) (*Client, string, error) {
	secret, err := k8sClient.CoreV1().Secrets(keycloak.Namespace).Get(keycloak.Spec.CredentialsSecret, meta_v1.GetOptions{})
	if err != nil {
		return nil, "", errors.Wrap(err, "failed to get credentials secret for keycloak ")
	}
	fmt.Println("got secret ", secret.Data, string(secret.Data["host"]))
	a := auth{
		pass:  string(secret.Data["user_passwd"]),
		user:  string(secret.Data["user_name"]),
		host:  strings.TrimSpace(string(secret.Data["url"])),
		token: keycloak.Spec.Token,
	}
	c := &Client{
		Requester: defaultRequester(),
		auth:      a,
	}
	token, err := c.authenticate()
	if err != nil {
		return nil, "", err
	}
	return c, token, nil
}

func NewClient(user, pass, host string) *Client {
	return &Client{
		auth:      auth{user: user, pass: pass, host: host},
		Requester: defaultRequester(),
	}
}

type auth struct {
	host  string
	user  string
	pass  string
	token string
}

type keycloakToken struct {
	Token string `json:"access_token"`
}

func (c *Client) token() (string, error) {
	if c.auth.token != "" {
		return c.auth.token, nil
	}
	token, err := c.authenticate()
	if err != nil {
		return "", errors.Wrap(err, "failed to authenticate against keycloak")
	}
	c.auth.token = token
	return token, nil
}

func (c *Client) authenticate() (string, error) {
	// login and get token set token

	if c.auth.host != "" && c.auth.host[:len(c.auth.host)-1] != "/" {
		c.auth.host = c.auth.host + "/"
	}
	location := c.auth.host + keycloakAuthURL
	fmt.Println("auth", location, c.auth.user, c.auth.pass)
	res, err := http.PostForm(location, url.Values{"client_id": {"admin-cli"}, "username": {c.auth.user}, "password": {c.auth.pass}, "grant_type": {"password"}})
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	if res.StatusCode != http.StatusOK {
		// error unexpected status code
	}
	kcToken := &keycloakToken{}
	if err := json.Unmarshal(data, kcToken); err != nil {
		return "", err
	}
	fmt.Println("keycloak res ", kcToken.Token, res.Status)
	return kcToken.Token, nil
}

func (c *Client) CreateRealm() error {
	fmt.Println("creating a realm")
	return nil
}
