package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	Group        = "aerogear.org"
	Version      = "v1alpha1"
	KeycloakKind = "Keycloak"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type KeycloakList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []Keycloak `json:"items"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// crd:gen:Kind=Keycloak:Group=aerogear.org
type Keycloak struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              KeycloakSpec  `json:"spec"`
	Status            GenericStatus `json:"status,omitempty"`
}

type KeycloakSpec struct {
	Version          string          `json:"version"`
	AdminCredentials string          `json:"adminCredentials"`
	Realms           []KeycloakRealm `json:"realms"`
}

type KeycloakRealm struct {
	Name      string      `json:"name"`
	AuthTypes []AuthTypes `json:"authTypes"`
	Users     []Users     `json:"users"`
	Clients   []Client    `json:"clients"`
}

type AuthTypes struct {
	Provider     string `json:"provider"`
	ClientID     string `json:"clientID"`
	ClientSecret string `json:"clientSecret"`
}

type Users struct {
	UserName     string   `json:"userName"`
	Roles        []string `json:"roles"`
	OutputSecret string   `json:"outputSecret"`
}

type Client struct {
	Name         string            `json:"name"`
	ClientType   string            `json:"clientType"`
	Config       map[string]string `json:"config"`
	OutputSecret string            `json:"outputSecret"`
}

type GenericStatus struct {
	Phase    StatusPhase `json:"phase"`
	Message  string      `json:"message"`
	Attempts int         `json:"attempts"`
}

type StatusPhase string

var (
	PhaseAccepted StatusPhase = "accepted"
	PhaseComplete StatusPhase = "complete"
	PhaseFailed   StatusPhase = "failed"
)
