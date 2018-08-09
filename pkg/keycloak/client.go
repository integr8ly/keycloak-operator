package keycloak

import (
	"net/http"

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
	username  string
	password  string
}

func (c *Client) DoesRealmExist(realmName string) (bool, error) {
	return false, nil
}

func defaultRequester() requester {
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	c := &http.Client{Timeout: time.Second * 10}
	return c
}

type KeycloakInterface interface {
	DoesRealmExist(realmName string) (bool, error)
}

type KeycloakClientFactory interface {
	AuthenticatedClient(kc v1alpha1.Keycloak, user, pass, url string) (KeycloakInterface, error)
}

type KeycloakFactory struct {
}

func (kf *KeycloakFactory) AuthenticatedClient(kc v1alpha1.Keycloak, user, pass, url string) (KeycloakInterface, error) {

	return &Client{}, nil
}
