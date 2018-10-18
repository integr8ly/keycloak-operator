package realm

import (
	"testing"

	"github.com/aerogear/keycloak-operator/pkg/apis/aerogear/v1alpha1"
	"github.com/aerogear/keycloak-operator/pkg/keycloak"
	"github.com/operator-framework/operator-sdk/pkg/sdk"
	"k8s.io/client-go/kubernetes/fake"
)

type phaseHandlerFunc func(*v1alpha1.KeycloakRealm) (*v1alpha1.KeycloakRealm, error)

type SDKExpectation struct {
	Create int
	Get    int
	List   int
	Update int
	Delete int
}

func TestPhaseHandlerInitialise(t *testing.T) {
	cases := []struct {
		Name          string
		Object        *v1alpha1.KeycloakRealm
		ExpectedPhase v1alpha1.StatusPhase
		ExpectedError string
		FakeClient    *fake.Clientset
		FakeSDK       keycloak.SdkCruder
		FakeKCF       keycloak.KeycloakClientFactory
	}{
		{
			Name: "Initialise from blank phase",
			Object: &v1alpha1.KeycloakRealm{
				Status: v1alpha1.KeycloakRealmStatus{
					Phase: "",
				},
			},
			ExpectedPhase: v1alpha1.PhaseAccepted,
			FakeClient:    fake.NewSimpleClientset(),
			FakeSDK:       &keycloak.SdkCruderMock{},
			FakeKCF: &keycloak.KeycloakClientFactoryMock{
				AuthenticatedClientFunc: func(kc v1alpha1.Keycloak) (keycloak.KeycloakInterface, error) {
					return &keycloak.KeycloakInterfaceMock{
						ListRealmsFunc: func() ([]*v1alpha1.KeycloakRealm, error) {
							return nil, nil
						},
					}, nil
				},
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.Name, func(t *testing.T) {
			phaseHandler := NewPhaseHandler(testCase.FakeClient, testCase.FakeSDK, "test-namespace", testCase.FakeKCF)
			result, err := phaseHandler.Initialise(testCase.Object)
			if err != nil && testCase.ExpectedError != "" {
				t.Fatalf("expected error: %v, got: %v", testCase.ExpectedError, err)
			}

			if result.Status.Phase != testCase.ExpectedPhase {
				t.Fatalf("expected phase: %v, got: %v", testCase.ExpectedPhase, result.Status.Phase)
			}
		})
	}
}

func TestPhaseHandlerAccepted(t *testing.T) {
	cases := []struct {
		Name          string
		Object        *v1alpha1.KeycloakRealm
		ExpectedPhase v1alpha1.StatusPhase
		ExpectedError string
		FakeClient    *fake.Clientset
		FakeSDK       keycloak.SdkCruder
		FakeKCF       keycloak.KeycloakClientFactory
	}{
		{
			Name: "Move to provision when keycloak instance found",
			Object: &v1alpha1.KeycloakRealm{
				Status: v1alpha1.KeycloakRealmStatus{
					Phase: v1alpha1.PhaseAccepted,
				},
			},
			ExpectedPhase: v1alpha1.PhaseProvision,
			FakeClient:    fake.NewSimpleClientset(),
			FakeSDK: &keycloak.SdkCruderMock{
				ListFunc: func(namespace string, into sdk.Object, opts ...sdk.ListOption) error {
					*into.(*v1alpha1.KeycloakList) = v1alpha1.KeycloakList{
						Items: []v1alpha1.Keycloak{
							{
								Status: v1alpha1.KeycloakStatus{
									GenericStatus: v1alpha1.GenericStatus{
										Phase: v1alpha1.PhaseComplete,
									},
								},
							},
						},
					}
					return nil
				},
			},
			FakeKCF: &keycloak.KeycloakClientFactoryMock{
				AuthenticatedClientFunc: func(kc v1alpha1.Keycloak) (keycloak.KeycloakInterface, error) {
					return &keycloak.KeycloakInterfaceMock{}, nil
				},
			},
		},
		{
			Name: "Stay accepted when keycloak instance found but not ready",
			Object: &v1alpha1.KeycloakRealm{
				Status: v1alpha1.KeycloakRealmStatus{
					Phase: v1alpha1.PhaseAccepted,
				},
			},
			ExpectedPhase: v1alpha1.PhaseAccepted,
			FakeClient:    fake.NewSimpleClientset(),
			FakeSDK: &keycloak.SdkCruderMock{
				UpdateFunc: func(object sdk.Object) error {
					return nil
				},
				ListFunc: func(namespace string, into sdk.Object, opts ...sdk.ListOption) error {
					*into.(*v1alpha1.KeycloakList) = v1alpha1.KeycloakList{
						Items: []v1alpha1.Keycloak{
							{
								Status: v1alpha1.KeycloakStatus{
									GenericStatus: v1alpha1.GenericStatus{
										Phase: v1alpha1.PhaseAwaitProvision,
									},
								},
							},
						},
					}
					return nil
				},
			},
			FakeKCF: &keycloak.KeycloakClientFactoryMock{
				AuthenticatedClientFunc: func(kc v1alpha1.Keycloak) (keycloak.KeycloakInterface, error) {
					return &keycloak.KeycloakInterfaceMock{}, nil
				},
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.Name, func(t *testing.T) {
			phaseHandler := NewPhaseHandler(testCase.FakeClient, testCase.FakeSDK, "test-namespace", testCase.FakeKCF)
			result, err := phaseHandler.Accepted(testCase.Object)
			if err != nil && testCase.ExpectedError != "" {
				t.Fatalf("expected error: %v, got: %v", testCase.ExpectedError, err)
			}

			if result.Status.Phase != testCase.ExpectedPhase {
				t.Fatalf("expected phase: %v, got: %v", testCase.ExpectedPhase, result.Status.Phase)
			}
		})
	}
}

func TestPhaseHandlerProvision(t *testing.T) {
	cases := []struct {
		Name          string
		Object        *v1alpha1.KeycloakRealm
		ExpectedPhase v1alpha1.StatusPhase
		ExpectedError string
		FakeClient    *fake.Clientset
		FakeSDK       keycloak.SdkCruder
		FakeKCF       keycloak.KeycloakClientFactory
	}{
		{
			Name: "Move to provisioned when keycloak realm already exists in keycloak",
			Object: &v1alpha1.KeycloakRealm{
				Status: v1alpha1.KeycloakRealmStatus{
					Phase: v1alpha1.PhaseProvision,
				},
				Spec: v1alpha1.KeycloakRealmSpec{
					KeycloakApiRealm: &v1alpha1.KeycloakApiRealm{
						Realm: "keycloak-realm",
					},
				},
			},
			ExpectedPhase: v1alpha1.PhaseReconcile,
			FakeClient:    fake.NewSimpleClientset(),
			FakeSDK: &keycloak.SdkCruderMock{
				GetFunc: func(object sdk.Object, opts ...sdk.GetOption) error {
					return nil
				},
			},
			FakeKCF: &keycloak.KeycloakClientFactoryMock{
				AuthenticatedClientFunc: func(kc v1alpha1.Keycloak) (keycloak.KeycloakInterface, error) {
					return &keycloak.KeycloakInterfaceMock{
						GetRealmFunc: func(name string) (*v1alpha1.KeycloakRealm, error) {
							return &v1alpha1.KeycloakRealm{
								Spec: v1alpha1.KeycloakRealmSpec{
									KeycloakApiRealm: &v1alpha1.KeycloakApiRealm{
										Realm: "keycloak-realm",
									},
								},
								Status: v1alpha1.KeycloakRealmStatus{
									Phase: v1alpha1.PhaseReconcile,
								},
							}, nil
						},
					}, nil
				},
			},
		},
		{
			Name: "Move to provisioned when keycloak realm is created in keycloak",
			Object: &v1alpha1.KeycloakRealm{
				Status: v1alpha1.KeycloakRealmStatus{
					Phase: v1alpha1.PhaseProvision,
				},
				Spec: v1alpha1.KeycloakRealmSpec{
					KeycloakApiRealm: &v1alpha1.KeycloakApiRealm{
						Realm: "keycloak-realm",
					},
				},
			},
			ExpectedPhase: v1alpha1.PhaseReconcile,
			FakeClient:    fake.NewSimpleClientset(),
			FakeSDK: &keycloak.SdkCruderMock{
				GetFunc: func(object sdk.Object, opts ...sdk.GetOption) error {
					return nil
				},
			},
			FakeKCF: &keycloak.KeycloakClientFactoryMock{
				AuthenticatedClientFunc: func(kc v1alpha1.Keycloak) (keycloak.KeycloakInterface, error) {
					return &keycloak.KeycloakInterfaceMock{
						GetRealmFunc: func(name string) (*v1alpha1.KeycloakRealm, error) {
							return nil, nil
						},
						CreateRealmFunc: func(realm *v1alpha1.KeycloakRealm) error {
							return nil
						},
					}, nil
				},
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.Name, func(t *testing.T) {
			phaseHandler := NewPhaseHandler(testCase.FakeClient, testCase.FakeSDK, "test-namespace", testCase.FakeKCF)
			result, err := phaseHandler.Provision(testCase.Object)
			if err != nil && testCase.ExpectedError != "" {
				t.Fatalf("expected error: %v, got: %v", testCase.ExpectedError, err)
			}

			if result.Status.Phase != testCase.ExpectedPhase {
				t.Fatalf("expected phase: %v, got: %v", testCase.ExpectedPhase, result.Status.Phase)
			}
		})
	}
}

func TestPhaseHandlerReconcile(t *testing.T) {
	pointerStringVal := "secret-name"
	cases := []struct {
		Name          string
		Object        *v1alpha1.KeycloakRealm
		ExpectedPhase v1alpha1.StatusPhase
		ExpectedError string
		FakeClient    *fake.Clientset
		FakeSDK       keycloak.SdkCruder
		FakeKCF       keycloak.KeycloakClientFactory
	}{
		{
			Name: "no errors in reconcile loop with no reconcile objects",
			Object: &v1alpha1.KeycloakRealm{
				Status: v1alpha1.KeycloakRealmStatus{
					Phase: v1alpha1.PhaseReconcile,
				},
				Spec: v1alpha1.KeycloakRealmSpec{
					KeycloakApiRealm: &v1alpha1.KeycloakApiRealm{
						Realm: "keycloak-realm",
					},
				},
			},
			ExpectedPhase: v1alpha1.PhaseReconcile,
			FakeClient:    fake.NewSimpleClientset(),
			FakeSDK: &keycloak.SdkCruderMock{
				GetFunc: func(object sdk.Object, opts ...sdk.GetOption) error {
					return nil
				},
			},
			FakeKCF: &keycloak.KeycloakClientFactoryMock{
				AuthenticatedClientFunc: func(kc v1alpha1.Keycloak) (keycloak.KeycloakInterface, error) {
					return &keycloak.KeycloakInterfaceMock{
						ListUsersFunc: func(realmName string) ([]*v1alpha1.KeycloakUser, error) {
							return nil, nil
						},
						ListClientsFunc: func(realmName string) ([]*v1alpha1.KeycloakClient, error) {
							return nil, nil
						},
						ListIdentityProvidersFunc: func(realmName string) ([]*v1alpha1.KeycloakIdentityProvider, error) {
							return nil, nil
						},
					}, nil
				},
			},
		},
		{
			Name: "no errors in reconcile loop with reconcile objects",
			Object: &v1alpha1.KeycloakRealm{
				Status: v1alpha1.KeycloakRealmStatus{
					Phase: v1alpha1.PhaseReconcile,
				},
				Spec: v1alpha1.KeycloakRealmSpec{
					KeycloakApiRealm: &v1alpha1.KeycloakApiRealm{
						Realm: "keycloak-realm",
						Users: []*v1alpha1.KeycloakUser{
							{
								OutputSecret: &pointerStringVal,
								KeycloakApiUser: &v1alpha1.KeycloakApiUser{
									Email: "test@example.com",
								},
							},
						},
						Clients: []*v1alpha1.KeycloakClient{
							{
								OutputSecret: &pointerStringVal,
								KeycloakApiClient: &v1alpha1.KeycloakApiClient{
									Name: "test-client",
								},
							},
						},
						IdentityProviders: []*v1alpha1.KeycloakIdentityProvider{
							{
								Alias: "test-id",
							},
						},
					},
				},
			},
			ExpectedPhase: v1alpha1.PhaseReconcile,
			FakeClient:    fake.NewSimpleClientset(),
			FakeSDK: &keycloak.SdkCruderMock{
				GetFunc: func(object sdk.Object, opts ...sdk.GetOption) error {
					return nil
				},
			},
			FakeKCF: &keycloak.KeycloakClientFactoryMock{
				AuthenticatedClientFunc: func(kc v1alpha1.Keycloak) (keycloak.KeycloakInterface, error) {
					return &keycloak.KeycloakInterfaceMock{
						GetClientSecretFunc: func(clientId string, realmName string) (string, error) {
							return "client-secret", nil
						},
						GetClientInstallFunc: func(clientId string, realmName string) ([]byte, error) {
							return []byte("{}"), nil
						},
						ListUsersFunc: func(realmName string) ([]*v1alpha1.KeycloakUser, error) {
							return []*v1alpha1.KeycloakUser{
								{
									KeycloakApiUser: &v1alpha1.KeycloakApiUser{
										Email: "test@example.com",
									},
								},
							}, nil
						},
						ListClientsFunc: func(realmName string) ([]*v1alpha1.KeycloakClient, error) {
							return []*v1alpha1.KeycloakClient{
								{
									OutputSecret: &pointerStringVal,
									KeycloakApiClient: &v1alpha1.KeycloakApiClient{
										Name: "test-client",
									},
								},
							}, nil
						},
						ListIdentityProvidersFunc: func(realmName string) ([]*v1alpha1.KeycloakIdentityProvider, error) {
							return []*v1alpha1.KeycloakIdentityProvider{
								{
									Alias:  "test-id",
									Config: map[string]string{},
								},
							}, nil
						},
						UpdateIdentityProviderFunc: func(specIdentityProvider *v1alpha1.KeycloakIdentityProvider, realmName string) error {
							return nil
						},
					}, nil
				},
			},
		},
		{
			Name: "no errors in reconcile loop with inequal reconcile objects",
			Object: &v1alpha1.KeycloakRealm{
				Status: v1alpha1.KeycloakRealmStatus{
					Phase: v1alpha1.PhaseReconcile,
				},
				Spec: v1alpha1.KeycloakRealmSpec{
					KeycloakApiRealm: &v1alpha1.KeycloakApiRealm{
						Realm: "keycloak-realm",
						Users: []*v1alpha1.KeycloakUser{
							{
								OutputSecret: &pointerStringVal,
								KeycloakApiUser: &v1alpha1.KeycloakApiUser{
									Email:         "test@example.com",
									EmailVerified: true,
								},
							},
						},
						Clients: []*v1alpha1.KeycloakClient{
							{
								OutputSecret: &pointerStringVal,
								KeycloakApiClient: &v1alpha1.KeycloakApiClient{
									Name:       "test-client",
									BearerOnly: true,
								},
							},
						},
						IdentityProviders: []*v1alpha1.KeycloakIdentityProvider{
							{
								Alias:       "test-id",
								DisplayName: "new-name",
							},
						},
					},
				},
			},
			ExpectedPhase: v1alpha1.PhaseReconcile,
			FakeClient:    fake.NewSimpleClientset(),
			FakeSDK: &keycloak.SdkCruderMock{
				GetFunc: func(object sdk.Object, opts ...sdk.GetOption) error {
					return nil
				},
			},
			FakeKCF: &keycloak.KeycloakClientFactoryMock{
				AuthenticatedClientFunc: func(kc v1alpha1.Keycloak) (keycloak.KeycloakInterface, error) {
					return &keycloak.KeycloakInterfaceMock{
						UpdateClientFunc: func(specClient *v1alpha1.KeycloakClient, realmName string) error {
							return nil
						},
						UpdateIdentityProviderFunc: func(specIdentityProvider *v1alpha1.KeycloakIdentityProvider, realmName string) error {
							return nil
						},
						UpdateUserFunc: func(specUser *v1alpha1.KeycloakUser, realmName string) error {
							return nil
						},
						GetClientSecretFunc: func(clientId string, realmName string) (string, error) {
							return "client-secret", nil
						},
						GetClientInstallFunc: func(clientId string, realmName string) ([]byte, error) {
							return []byte("{}"), nil
						},
						ListUsersFunc: func(realmName string) ([]*v1alpha1.KeycloakUser, error) {
							return []*v1alpha1.KeycloakUser{
								{
									KeycloakApiUser: &v1alpha1.KeycloakApiUser{
										Email: "test@example.com",
									},
								},
							}, nil
						},
						ListClientsFunc: func(realmName string) ([]*v1alpha1.KeycloakClient, error) {
							return []*v1alpha1.KeycloakClient{
								{
									OutputSecret: &pointerStringVal,
									KeycloakApiClient: &v1alpha1.KeycloakApiClient{
										Name: "test-client",
									},
								},
							}, nil
						},
						ListIdentityProvidersFunc: func(realmName string) ([]*v1alpha1.KeycloakIdentityProvider, error) {
							return []*v1alpha1.KeycloakIdentityProvider{
								{
									Alias:  "test-id",
									Config: map[string]string{},
								},
							}, nil
						},
					}, nil
				},
			},
		},
		{
			Name: "no errors in reconcile loop with reconcile objects in CR and not in keycloak",
			Object: &v1alpha1.KeycloakRealm{
				Status: v1alpha1.KeycloakRealmStatus{
					Phase: v1alpha1.PhaseReconcile,
				},
				Spec: v1alpha1.KeycloakRealmSpec{
					KeycloakApiRealm: &v1alpha1.KeycloakApiRealm{
						Realm: "keycloak-realm",
						Users: []*v1alpha1.KeycloakUser{
							{
								OutputSecret: &pointerStringVal,
								KeycloakApiUser: &v1alpha1.KeycloakApiUser{
									Email: "test@example.com",
								},
							},
						},
						Clients: []*v1alpha1.KeycloakClient{
							{
								OutputSecret: &pointerStringVal,
								KeycloakApiClient: &v1alpha1.KeycloakApiClient{
									Name: "test-client",
								},
							},
						},
						IdentityProviders: []*v1alpha1.KeycloakIdentityProvider{
							{
								Alias: "test-id",
							},
						},
					},
				},
			},
			ExpectedPhase: v1alpha1.PhaseReconcile,
			FakeClient:    fake.NewSimpleClientset(),
			FakeSDK: &keycloak.SdkCruderMock{
				GetFunc: func(object sdk.Object, opts ...sdk.GetOption) error {
					return nil
				},
			},
			FakeKCF: &keycloak.KeycloakClientFactoryMock{
				AuthenticatedClientFunc: func(kc v1alpha1.Keycloak) (keycloak.KeycloakInterface, error) {
					return &keycloak.KeycloakInterfaceMock{
						CreateClientFunc: func(client *v1alpha1.KeycloakClient, realmName string) error {
							return nil
						},
						CreateIdentityProviderFunc: func(identityProvider *v1alpha1.KeycloakIdentityProvider, realmName string) error {
							return nil
						},
						CreateUserFunc: func(user *v1alpha1.KeycloakUser, realmName string) error {
							return nil
						},
						FindUserByEmailFunc: func(email string, realm string) (*v1alpha1.KeycloakApiUser, error) {
							return &v1alpha1.KeycloakApiUser{
								Email: "test@example.com",
							}, nil
						},
						UpdatePasswordFunc: func(user *v1alpha1.KeycloakApiUser, realmName string, newPass string) error {
							return nil
						},
						GetClientSecretFunc: func(clientId string, realmName string) (string, error) {
							return "client-secret", nil
						},
						GetClientInstallFunc: func(clientId string, realmName string) ([]byte, error) {
							return []byte("{}"), nil
						},
						ListUsersFunc: func(realmName string) ([]*v1alpha1.KeycloakUser, error) {
							return []*v1alpha1.KeycloakUser{}, nil
						},
						ListClientsFunc: func(realmName string) ([]*v1alpha1.KeycloakClient, error) {
							return []*v1alpha1.KeycloakClient{}, nil
						},
						ListIdentityProvidersFunc: func(realmName string) ([]*v1alpha1.KeycloakIdentityProvider, error) {
							return []*v1alpha1.KeycloakIdentityProvider{}, nil
						},
						UpdateIdentityProviderFunc: func(specIdentityProvider *v1alpha1.KeycloakIdentityProvider, realmName string) error {
							return nil
						},
					}, nil
				},
			},
		},
		{
			Name: "no errors when identity provider in keycloak, but not in CR",
			Object: &v1alpha1.KeycloakRealm{
				Status: v1alpha1.KeycloakRealmStatus{
					Phase: v1alpha1.PhaseReconcile,
				},
				Spec: v1alpha1.KeycloakRealmSpec{
					KeycloakApiRealm: &v1alpha1.KeycloakApiRealm{
						Realm: "keycloak-realm",
					},
				},
			},
			ExpectedPhase: v1alpha1.PhaseReconcile,
			FakeClient:    fake.NewSimpleClientset(),
			FakeSDK: &keycloak.SdkCruderMock{
				GetFunc: func(object sdk.Object, opts ...sdk.GetOption) error {
					return nil
				},
			},
			FakeKCF: &keycloak.KeycloakClientFactoryMock{
				AuthenticatedClientFunc: func(kc v1alpha1.Keycloak) (keycloak.KeycloakInterface, error) {
					return &keycloak.KeycloakInterfaceMock{
						DeleteIdentityProviderFunc: func(alias string, realmName string) error {
							return nil
						},
						ListUsersFunc: func(realmName string) ([]*v1alpha1.KeycloakUser, error) {
							return []*v1alpha1.KeycloakUser{}, nil
						},
						ListClientsFunc: func(realmName string) ([]*v1alpha1.KeycloakClient, error) {
							return []*v1alpha1.KeycloakClient{}, nil
						},
						ListIdentityProvidersFunc: func(realmName string) ([]*v1alpha1.KeycloakIdentityProvider, error) {
							return []*v1alpha1.KeycloakIdentityProvider{
								{
									Alias:  "test-id",
									Config: map[string]string{},
								},
							}, nil
						},
					}, nil
				},
			},
		}, {
			Name: "no errors when user in keycloak, but not in CR",
			Object: &v1alpha1.KeycloakRealm{
				Status: v1alpha1.KeycloakRealmStatus{
					Phase: v1alpha1.PhaseReconcile,
				},
				Spec: v1alpha1.KeycloakRealmSpec{
					KeycloakApiRealm: &v1alpha1.KeycloakApiRealm{
						Realm: "keycloak-realm",
					},
				},
			},
			ExpectedPhase: v1alpha1.PhaseReconcile,
			FakeClient:    fake.NewSimpleClientset(),
			FakeSDK: &keycloak.SdkCruderMock{
				GetFunc: func(object sdk.Object, opts ...sdk.GetOption) error {
					return nil
				},
			},
			FakeKCF: &keycloak.KeycloakClientFactoryMock{
				AuthenticatedClientFunc: func(kc v1alpha1.Keycloak) (keycloak.KeycloakInterface, error) {
					return &keycloak.KeycloakInterfaceMock{
						DeleteUserFunc: func(userID string, realmName string) error {
							return nil
						},
						ListUsersFunc: func(realmName string) ([]*v1alpha1.KeycloakUser, error) {
							return []*v1alpha1.KeycloakUser{
								{
									KeycloakApiUser: &v1alpha1.KeycloakApiUser{
										Email: "test@example.com",
									},
								},
							}, nil
						},
						ListClientsFunc: func(realmName string) ([]*v1alpha1.KeycloakClient, error) {
							return []*v1alpha1.KeycloakClient{}, nil
						},
						ListIdentityProvidersFunc: func(realmName string) ([]*v1alpha1.KeycloakIdentityProvider, error) {
							return []*v1alpha1.KeycloakIdentityProvider{}, nil
						},
					}, nil
				},
			},
		},
		{
			Name: "no errors when client in keycloak, but not in CR",
			Object: &v1alpha1.KeycloakRealm{
				Status: v1alpha1.KeycloakRealmStatus{
					Phase: v1alpha1.PhaseReconcile,
				},
				Spec: v1alpha1.KeycloakRealmSpec{
					KeycloakApiRealm: &v1alpha1.KeycloakApiRealm{
						Realm: "keycloak-realm",
					},
				},
			},
			ExpectedPhase: v1alpha1.PhaseReconcile,
			FakeClient:    fake.NewSimpleClientset(),
			FakeSDK: &keycloak.SdkCruderMock{
				GetFunc: func(object sdk.Object, opts ...sdk.GetOption) error {
					return nil
				},
			},
			FakeKCF: &keycloak.KeycloakClientFactoryMock{
				AuthenticatedClientFunc: func(kc v1alpha1.Keycloak) (keycloak.KeycloakInterface, error) {
					return &keycloak.KeycloakInterfaceMock{
						DeleteClientFunc: func(clientID string, realmName string) error {
							return nil
						},
						ListUsersFunc: func(realmName string) ([]*v1alpha1.KeycloakUser, error) {
							return []*v1alpha1.KeycloakUser{}, nil
						},
						ListClientsFunc: func(realmName string) ([]*v1alpha1.KeycloakClient, error) {
							return []*v1alpha1.KeycloakClient{
								{
									OutputSecret: &pointerStringVal,
									KeycloakApiClient: &v1alpha1.KeycloakApiClient{
										Name: "test-client",
									},
								},
							}, nil
						},
						ListIdentityProvidersFunc: func(realmName string) ([]*v1alpha1.KeycloakIdentityProvider, error) {
							return []*v1alpha1.KeycloakIdentityProvider{}, nil
						},
					}, nil
				},
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.Name, func(t *testing.T) {
			phaseHandler := NewPhaseHandler(testCase.FakeClient, testCase.FakeSDK, "test-namespace", testCase.FakeKCF)
			result, err := phaseHandler.Reconcile(testCase.Object)
			if err != nil && testCase.ExpectedError != "" {
				t.Fatalf("expected error: %v, got: %v", testCase.ExpectedError, err)
			}

			if result.Status.Phase != testCase.ExpectedPhase {
				t.Fatalf("expected phase: %v, got: %v", testCase.ExpectedPhase, result.Status.Phase)
			}
		})
	}
}

func TestProvisionDeletesPassword(t *testing.T) {
	pointerStringVal := "secret-name"
	cases := []struct {
		Name          string
		Object        *v1alpha1.KeycloakRealm
		ExpectedPhase v1alpha1.StatusPhase
		ExpectedError string
		FakeClient    *fake.Clientset
		FakeSDK       keycloak.SdkCruder
		FakeKCF       keycloak.KeycloakClientFactory
	}{
		{
			Name: "Password is removed from user object",
			Object: &v1alpha1.KeycloakRealm{
				Status: v1alpha1.KeycloakRealmStatus{
					Phase: v1alpha1.PhaseReconcile,
				},
				Spec: v1alpha1.KeycloakRealmSpec{
					KeycloakApiRealm: &v1alpha1.KeycloakApiRealm{
						Realm: "keycloak-realm",
						Users: []*v1alpha1.KeycloakUser{
							{
								OutputSecret: &pointerStringVal,
								Password:     &pointerStringVal,
								KeycloakApiUser: &v1alpha1.KeycloakApiUser{
									Email:         "test@example.com",
									EmailVerified: true,
								},
							},
						},
						Clients:           []*v1alpha1.KeycloakClient{},
						IdentityProviders: []*v1alpha1.KeycloakIdentityProvider{},
					},
				},
			},
			ExpectedPhase: v1alpha1.PhaseReconcile,
			FakeClient:    fake.NewSimpleClientset(),
			FakeSDK: &keycloak.SdkCruderMock{
				GetFunc: func(object sdk.Object, opts ...sdk.GetOption) error {
					return nil
				},
			},
			FakeKCF: &keycloak.KeycloakClientFactoryMock{
				AuthenticatedClientFunc: func(kc v1alpha1.Keycloak) (keycloak.KeycloakInterface, error) {
					return &keycloak.KeycloakInterfaceMock{
						CreateUserFunc: func(specUser *v1alpha1.KeycloakUser, realmName string) error {
							return nil
						},
						FindUserByEmailFunc: func(email string, realm string) (*v1alpha1.KeycloakApiUser, error) {
							return &v1alpha1.KeycloakApiUser{
								Email:         "test@example.com",
								EmailVerified: true,
							}, nil
						},
						ListUsersFunc: func(realmName string) ([]*v1alpha1.KeycloakUser, error) {
							return []*v1alpha1.KeycloakUser{}, nil
						},
						UpdatePasswordFunc: func(user *v1alpha1.KeycloakApiUser, realmName string, newPass string) error {
							return nil
						},
						ListClientsFunc: func(realmName string) ([]*v1alpha1.KeycloakClient, error) {
							return []*v1alpha1.KeycloakClient{}, nil
						},
						ListIdentityProvidersFunc: func(realmName string) ([]*v1alpha1.KeycloakIdentityProvider, error) {
							return []*v1alpha1.KeycloakIdentityProvider{}, nil
						},
					}, nil
				},
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.Name, func(t *testing.T) {
			phaseHandler := NewPhaseHandler(testCase.FakeClient, testCase.FakeSDK, "test-namespace", testCase.FakeKCF)
			_, err := phaseHandler.Reconcile(testCase.Object)
			if err != nil && testCase.ExpectedError != "" {
				t.Fatalf("expected error: %v, got: %v", testCase.ExpectedError, err)
			}
			for _, user := range testCase.Object.Spec.Users {
				if user.Password != nil {
					t.Fatalf("expected password to be nil, got: %v", *user.Password)
				}
			}

		})
	}
}

func TestPhaseHandlerDeprovision(t *testing.T) {
	secretString := "secret"
	cases := []struct {
		Name          string
		Object        *v1alpha1.KeycloakRealm
		ExpectedPhase v1alpha1.StatusPhase
		ExpectedError string
		FakeClient    *fake.Clientset
		FakeSDK       keycloak.SdkCruder
		FakeKCF       keycloak.KeycloakClientFactory
	}{
		{
			Name: "Deprovision realm",
			Object: &v1alpha1.KeycloakRealm{
				Status: v1alpha1.KeycloakRealmStatus{
					Phase: v1alpha1.PhaseDeprovisioning,
				},
				Spec: v1alpha1.KeycloakRealmSpec{
					KeycloakApiRealm: &v1alpha1.KeycloakApiRealm{
						Clients:           []*v1alpha1.KeycloakClient{},
						Users:             []*v1alpha1.KeycloakUser{},
						IdentityProviders: []*v1alpha1.KeycloakIdentityProvider{},
					},
				},
			},
			ExpectedPhase: v1alpha1.PhaseDeprovisioning,
			FakeClient:    fake.NewSimpleClientset(),
			FakeSDK: &keycloak.SdkCruderMock{
				GetFunc: func(object sdk.Object, opts ...sdk.GetOption) error {
					return nil
				},
			},
			FakeKCF: &keycloak.KeycloakClientFactoryMock{
				AuthenticatedClientFunc: func(kc v1alpha1.Keycloak) (keycloak.KeycloakInterface, error) {
					return &keycloak.KeycloakInterfaceMock{
						DeleteRealmFunc: func(realmName string) error {
							return nil
						},
					}, nil
				},
			},
		},
		{
			Name: "Deprovision realm with objects",
			Object: &v1alpha1.KeycloakRealm{
				Status: v1alpha1.KeycloakRealmStatus{
					Phase: v1alpha1.PhaseDeprovisioning,
				},
				Spec: v1alpha1.KeycloakRealmSpec{
					KeycloakApiRealm: &v1alpha1.KeycloakApiRealm{
						Clients: []*v1alpha1.KeycloakClient{
							{
								OutputSecret: &secretString,
							},
						},
						Users: []*v1alpha1.KeycloakUser{
							{
								OutputSecret: &secretString,
							},
						},
						IdentityProviders: []*v1alpha1.KeycloakIdentityProvider{},
					},
				},
			},
			ExpectedPhase: v1alpha1.PhaseDeprovisioning,
			FakeClient:    fake.NewSimpleClientset(),
			FakeSDK: &keycloak.SdkCruderMock{
				DeleteFunc: func(object sdk.Object, opts ...sdk.DeleteOption) error {
					return nil
				},
				GetFunc: func(object sdk.Object, opts ...sdk.GetOption) error {
					return nil
				},
			},
			FakeKCF: &keycloak.KeycloakClientFactoryMock{
				AuthenticatedClientFunc: func(kc v1alpha1.Keycloak) (keycloak.KeycloakInterface, error) {
					return &keycloak.KeycloakInterfaceMock{
						DeleteRealmFunc: func(realmName string) error {
							return nil
						},
					}, nil
				},
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.Name, func(t *testing.T) {
			phaseHandler := NewPhaseHandler(testCase.FakeClient, testCase.FakeSDK, "test-namespace", testCase.FakeKCF)
			result, err := phaseHandler.Deprovision(testCase.Object)
			if err != nil && testCase.ExpectedError != "" {
				t.Fatalf("expected error: %v, got: %v", testCase.ExpectedError, err)
			}

			if result.Status.Phase != testCase.ExpectedPhase {
				t.Fatalf("expected phase: %v, got: %v", testCase.ExpectedPhase, result.Status.Phase)
			}
		})
	}
}
