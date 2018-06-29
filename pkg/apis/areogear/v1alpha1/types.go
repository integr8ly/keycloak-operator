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
	Spec              KeycloakRealmSpec   `json:"spec"`
	Status            KeycloakRealmStatus `json:"status,omitempty"`
}

type KeycloakRealmSpec struct {
	RealmName  string `json:"realmName"`
	KeycloakID string `json:"keycloakID"`
	User       string `json:"user"`
}
type KeycloakRealmStatus struct {
	Ready bool `json:"ready"`
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
	Spec              KeycloakSpec   `json:"spec"`
	Status            KeycloakStatus `json:"status,omitempty"`
}

type KeycloakSpec struct {
	URL               string `json:"url"`
	CredentialsSecret string `json:"credentialsSecret"`
}
type KeycloakStatus struct {
	Ready bool `json:"ready"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type KeycloakClient struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              KeycloakClientSpec   `json:"spec"`
	Status            KeycloakClientStatus `json:"status,omitempty"`
}

type KeycloakClientSpec struct {
	KeycloakID        string `json:"keycloakID"`
	Type              string `json:"type"`
	Realm             string `json:"realm"`
	CredentialsSecret string `json:"credentialsSecret"`
}
type KeycloakClientStatus struct {
	Ready bool `json:"ready"`
}
