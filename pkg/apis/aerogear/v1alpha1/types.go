package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	Group             = "aerogear.org"
	Version           = "v1alpha1"
	KeycloakKind      = "Keycloak"
	KeycloakVersion   = "4.1.0"
	KeycloakFinalizer = "finalizer.org.aerogear.keycloak"
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
	Spec              KeycloakSpec   `json:"spec"`
	Status            KeycloakStatus `json:"status,omitempty"`
}

func (k *Keycloak) Defaults() {

}

type KeycloakSpec struct {
	Version          string          `json:"version"`
	InstanceName     string          `json:"instanceName"`
	InstanceUID      string          `json:"instanceUID"`
	AdminCredentials string          `json:"adminCredentials"`
	Realms           []KeycloakRealm `json:"realms"`
}

type KeycloakRealm struct {
	ID                string                      `json:"id,omitempty"`
	Realm             string                      `json:"realm"`
	Enabled           bool                        `json:"enabled"`
	DisplayName       string                      `json:"displayName"`
	Users             []KeycloakUser              `json:"users"`
	Clients           []KeycloakClient            `json:"clients"`
	IdentityProviders []KeycloakIdentityProvider `json:"identityProviders"`
}

type KeycloakIdentityProvider struct {
	Alias                     string            `json:"alias"`
	DisplayName               string            `json:"displayName"`
	InternalID                string            `json:"internalId"`
	ProviderID                string            `json:"providerId"`
	Enabled                   bool              `json:"enabled"`
	TrustEmail                bool              `json:"trustEmail"`
	StoreToken                bool              `json:"storeToken"`
	AddReadTokenRoleOnCreate  bool              `json:"addReadTokenRoleOnCreate"`
	FirstBrokerLoginFlowAlias string            `json:"firstBrokerLoginFlowAlias"`
	PostBrokerLoginFlowAlias  string            `json:"postBrokerLoginFlowAlias"`
	Config                    map[string]string `json:"config"`
}

type KeycloakUser struct {
	UserName        string              `json:"username"`
	FirstName       string              `json:"firstName"`
	LastName        string              `json:"lastName"`
	Email           string              `json:"email"`
	EmailVerified   bool                `json:"emailVerified"`
	Enabled         bool                `json:"enabled"`
	RealmRoles      []string            `json:"realmRoles"`
	ClientRoles     map[string][]string `json:"clientRoles"`
	RequiredActions []string            `json:"requiredActions"`
	Groups          []string            `json:"groups"`
}

type KeycloakProtocolMapper struct {
	ID              string            `json:"id,omitempty"`
	Name            string            `json:"name"`
	Protocol        string            `json:"protocol"`
	ProtocolMapper  string            `json:"protocolMapper"`
	ConsentRequired bool              `json:"consentRequired"`
	ConsentText     string            `json:"consentText,omitempty"`
	Config          map[string]string `json:"config"`
}

type KeycloakClient struct {
	ID                        string                   `json:"id,omitempty"`
	ClientID                  string                   `json:"clientId"`
	Name                      string                   `json:"name"`
	BaseURL                   string                   `json:"baseUrl,omitempty"`
	SurrogateAuthRequired     bool                     `json:"surrogateAuthRequired"`
	Enabled                   bool                     `json:"enabled"`
	ClientAuthenticatorType   string                   `json:"clientAuthenticatorType"`
	DefaultRoles              []string                 `json:"defaultRoles,omitempty"`
	RedirectUris              []string                 `json:"redirectUris"`
	WebOrigins                []string                 `json:"webOrigins"`
	NotBefore                 int                      `json:"notBefore"`
	BearerOnly                bool                     `json:"bearerOnly"`
	ConsentRequired           bool                     `json:"consentRequired"`
	StandardFlowEnabled       bool                     `json:"standardFlowEnabled"`
	ImplicitFlowEnabled       bool                     `json:"implicitFlowEnabled"`
	DirectAccessGrantsEnabled bool                     `json:"directAccessGrantsEnabled"`
	ServiceAccountsEnabled    bool                     `json:"serviceAccountsEnabled"`
	PublicClient              bool                     `json:"publicClient"`
	FrontchannelLogout        bool                     `json:"frontchannelLogout"`
	Protocol                  string                   `json:"protocol,omitempty"`
	Attributes                map[string]string        `json:"attributes"`
	FullScopeAllowed          bool                     `json:"fullScopeAllowed"`
	NodeReRegistrationTimeout int                      `json:"nodeReRegistrationTimeout"`
	ProtocolMappers           []KeycloakProtocolMapper `json:"protocolMappers"`
	UseTemplateConfig         bool                     `json:"useTemplateConfig"`
	UseTemplateScope          bool                     `json:"useTemplateScope"`
	UseTemplateMappers        bool                     `json:"useTemplateMappers"`
	Access                    map[string]string        `json:"access"`
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
