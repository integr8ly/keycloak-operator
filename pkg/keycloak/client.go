package keycloak

import (
	"net/http"

	"time"

	"crypto/tls"
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
}

func defaultRequester() requester {
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	c := &http.Client{Timeout: time.Second * 10}
	return c
}
