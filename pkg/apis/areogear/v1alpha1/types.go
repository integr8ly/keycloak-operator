package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type KeycloakRealmList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []KeycloakRealm `json:"items"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type KeycloakRealm struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              KeycloakRealmSpec `json:"spec"`
	Status            GenericStatus     `json:"status,omitempty"`
}

type KeycloakRealmSpec struct {
	RealmName  string `json:"realmName"`
	KeycloakID string `json:"keycloakID"`
	User       string `json:"user"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type KeycloakList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []Keycloak `json:"items"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type Keycloak struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              KeycloakSpec  `json:"spec"`
	Status            GenericStatus `json:"status,omitempty"`
}

type KeycloakSpec struct {
	URL               string `json:"url"`
	CredentialsSecret string `json:"credentialsSecret"`
	Token             string `json:"token"` //todo review should it be in the secret
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type KeycloakClientList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []KeycloakClient `json:"items"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type KeycloakClient struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              KeycloakClientSpec `json:"spec"`
	Status            GenericStatus      `json:"status,omitempty"`
}

type KeycloakClientSpec struct {
	KeycloakID        string `json:"keycloakID"`
	Type              string `json:"type"`
	Realm             string `json:"realm"`
	CredentialsSecret string `json:"credentialsSecret"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type KeycloakClientSyncList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []KeycloakClientSync `json:"items"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type KeycloakClientSync struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              KeycloakClientSyncSpec `json:"spec"`
	Status            GenericStatus          `json:"status,omitempty"`
}

type KeycloakClientSyncSpec struct {
	KeycloakID string `json:"keycloakID"`
	ClientType string `json:"type"`
	Realm      string `json:"realm"`
}

type GenericStatus struct {
	Phase    StatusPhase `json:"phase"`
	Message  string      `json:"message"`
	Attempts int         `json:"attempts"`
}

type StatusPhase string

var (
	PhaseAccepted   StatusPhase = "accepted"
	PhaseComplete   StatusPhase = "complete"
	PhaseFailed     StatusPhase = "failed"
	PhaseAuthFailed StatusPhase = "authfailed"
)
