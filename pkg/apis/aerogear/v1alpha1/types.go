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

type Config struct {
	ResyncPeriod  int
	LogLevel      string
	SyncResources bool
}

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
	ID                string                     `json:"id,omitempty"`
	Realm             string                     `json:"realm,omitempty"`
	Enabled           bool                       `json:"enabled,omitempty"`
	DisplayName       string                     `json:"displayName,omitempty"`
	Users             []KeycloakUser             `json:"users,omitempty"`
	Clients           []KeycloakClient           `json:"clients,omitempty"`
	IdentityProviders []KeycloakIdentityProvider `json:"identityProviders,omitempty"`
}

type KeycloakIdentityProvider struct {
	Alias                     string            `json:"alias,omitempty"`
	DisplayName               string            `json:"displayName,omitempty"`
	InternalID                string            `json:"internalId,omitempty"`
	ProviderID                string            `json:"providerId,omitempty"`
	Enabled                   bool              `json:"enabled"`
	TrustEmail                bool              `json:"trustEmail"`
	StoreToken                bool              `json:"storeToken"`
	AddReadTokenRoleOnCreate  bool              `json:"addReadTokenRoleOnCreate"`
	FirstBrokerLoginFlowAlias string            `json:"firstBrokerLoginFlowAlias,omitempty"`
	PostBrokerLoginFlowAlias  string            `json:"postBrokerLoginFlowAlias,omitempty"`
	Config                    map[string]string `json:"config,omitempty"`
}
type KeycloakIdentityProviderPair struct {
	KcIdentityProvider   *KeycloakIdentityProvider
	SpecIdentityProvider *KeycloakIdentityProvider
}

type KeycloakUser struct {
	ID              string              `json:"id,omitempty"`
	UserName        string              `json:"username,omitempty"`
	FirstName       string              `json:"firstName,omitempty"`
	LastName        string              `json:"lastName,omitempty"`
	Email           string              `json:"email,omitempty"`
	EmailVerified   bool                `json:"emailVerified"`
	Enabled         bool                `json:"enabled"`
	RealmRoles      []string            `json:"realmRoles,omitempty"`
	ClientRoles     map[string][]string `json:"clientRoles,omitempty"`
	RequiredActions []string            `json:"requiredActions,omitempty"`
	Groups          []string            `json:"groups,omitempty"`
}

type KeycloakUserPair struct {
	KcUser   *KeycloakUser
	SpecUser *KeycloakUser
}

type KeycloakProtocolMapper struct {
	ID              string            `json:"id,omitempty"`
	Name            string            `json:"name,omitempty"`
	Protocol        string            `json:"protocol,omitempty"`
	ProtocolMapper  string            `json:"protocolMapper,omitempty"`
	ConsentRequired bool              `json:"consentRequired,omitempty"`
	ConsentText     string            `json:"consentText,omitempty"`
	Config          map[string]string `json:"config,omitempty"`
}

type KeycloakClient struct {
	ID                        string                   `json:"id,omitempty"`
	ClientID                  string                   `json:"clientId,omitempty"`
	Name                      string                   `json:"name,omitempty"`
	BaseURL                   string                   `json:"baseUrl,omitempty"`
	SurrogateAuthRequired     bool                     `json:"surrogateAuthRequired"`
	Enabled                   bool                     `json:"enabled"`
	ClientAuthenticatorType   string                   `json:"clientAuthenticatorType,omitempty"`
	DefaultRoles              []string                 `json:"defaultRoles,omitempty,omitempty"`
	RedirectUris              []string                 `json:"redirectUris,omitempty"`
	WebOrigins                []string                 `json:"webOrigins,omitempty"`
	NotBefore                 int                      `json:"notBefore,omitempty"`
	BearerOnly                bool                     `json:"bearerOnly"`
	ConsentRequired           bool                     `json:"consentRequired"`
	StandardFlowEnabled       bool                     `json:"standardFlowEnabled"`
	ImplicitFlowEnabled       bool                     `json:"implicitFlowEnabled"`
	DirectAccessGrantsEnabled bool                     `json:"directAccessGrantsEnabled"`
	ServiceAccountsEnabled    bool                     `json:"serviceAccountsEnabled"`
	PublicClient              bool                     `json:"publicClient"`
	FrontchannelLogout        bool                     `json:"frontchannelLogout"`
	Protocol                  string                   `json:"protocol,omitempty"`
	Attributes                map[string]string        `json:"attributes,omitempty"`
	FullScopeAllowed          bool                     `json:"fullScopeAllowed"`
	NodeReRegistrationTimeout int                      `json:"nodeReRegistrationTimeout,omitempty"`
	ProtocolMappers           []KeycloakProtocolMapper `json:"protocolMappers,omitempty"`
	UseTemplateConfig         bool                     `json:"useTemplateConfig"`
	UseTemplateScope          bool                     `json:"useTemplateScope"`
	UseTemplateMappers        bool                     `json:"useTemplateMappers"`
	Access                    map[string]bool          `json:"access,omitempty"`
}
type KeycloakClientPair struct {
	KcClient   *KeycloakClient
	SpecClient *KeycloakClient
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
