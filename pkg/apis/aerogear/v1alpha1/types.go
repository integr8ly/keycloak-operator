package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	Group        = "aerogear.org"
	Version      = "v1alpha1"
	KeycloakKind = "Keycloak"
	KeycloakVersion = "4.1.0"
	KeycloakFinalizer = "finalizer.org.aerogrear.keycloak"
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
	Status            KeycloakStatus `json:"status,omitempty"`
}

func (k *Keycloak) Defaults() {

}

type KeycloakSpec struct {
	Version          string          `json:"version"`
	InstanceID       string          `json:"instanceID"`
	AdminCredentials string          `json:"adminCredentials"`
	Realms           []KeycloakRealm `json:"realms"`
}

type KeycloakRealm struct {
	Name      string           `json:"name"`
	AuthTypes []AuthTypes      `json:"authMethods"`
	Users     []KeycloakUser   `json:"users"`
	Clients   []KeycloakClient `json:"clients"`
}

type AuthTypes struct {
	Provider     string `json:"provider"`
	ClientID     string `json:"clientID"`
	ClientSecret string `json:"clientSecret"`
}

type KeycloakUser struct {
	UserName     string   `json:"userName"`
	Roles        []string `json:"roles"`
	OutputSecret string   `json:"outputSecret"`
}

type KeycloakClient struct {
	Name         string            `json:"name"`
	ClientType   string            `json:"clientType"`
	Config       map[string]string `json:"config"`
	OutputSecret string            `json:"outputSecret"`
}

type GenericStatus struct {
	Phase    StatusPhase `json:"phase"`
	Message  string      `json:"message"`
	Attempts int         `json:"attempts"`
	// marked as true when all work is done on it
	Ready bool `json:"ready"`
}

type KeycloakStatus struct {
	GenericStatus
	SharedConfig StatusSharedConfig `json:"sharedConfig"`
}

type StatusPhase string

var (
	NoPhase                 StatusPhase = ""
	PhaseAccepted           StatusPhase = "accepted"
	PhaseComplete           StatusPhase = "complete"
	PhaseFailed             StatusPhase = "failed"
	PhaseModified           StatusPhase = "modified"
	PhaseProvisioning       StatusPhase = "provisioning"
	PhaseDeprovisioning     StatusPhase = "deprovisioning"
	PhaseDeprovisioned      StatusPhase = "deprovisioned"
	PhaseDeprovisionFailed  StatusPhase = "deprovisionFailed"
	PhaseCredentialsPending StatusPhase = "credentialsPending"
	PhaseCredentialsCreated StatusPhase = "credentialsCreated"
)
