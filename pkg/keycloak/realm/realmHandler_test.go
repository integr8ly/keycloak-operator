package realm

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/integr8ly/keycloak-operator/pkg/keycloak"
	"github.com/operator-framework/operator-sdk/pkg/sdk"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/integr8ly/keycloak-operator/pkg/apis/aerogear/v1alpha1"
)

func TestRealmHandler(t *testing.T) {
	now := metav1.NewTime(time.Now())
	cases := []struct {
		Name          string
		Context       context.Context
		Object        *v1alpha1.KeycloakRealm
		Deleted       bool
		ExpectedPhase v1alpha1.StatusPhase
		ExpectedError string
		FakeClient    *fake.Clientset
		FakeSDK       keycloak.SdkCruder
		FakeKCF       keycloak.KeycloakClientFactory
		FakeHandler   Handler
	}{
		{
			Name:    "No error when deleted is true",
			Context: context.TODO(),
			Object: &v1alpha1.KeycloakRealm{
				Status: v1alpha1.KeycloakRealmStatus{
					Phase: v1alpha1.NoPhase,
				},
			},
			Deleted:       true,
			ExpectedPhase: v1alpha1.NoPhase,
			FakeClient:    fake.NewSimpleClientset(),
			FakeSDK:       &keycloak.SdkCruderMock{},
			FakeKCF:       &keycloak.KeycloakClientFactoryMock{},
			FakeHandler:   &HandlerMock{},
			ExpectedError: "",
		},
		{
			Name:    "No error when deletion timestamp is set",
			Context: context.TODO(),
			Object: &v1alpha1.KeycloakRealm{
				Status: v1alpha1.KeycloakRealmStatus{
					Phase: v1alpha1.NoPhase,
				},
				ObjectMeta: metav1.ObjectMeta{
					DeletionTimestamp: &now,
				},
			},
			ExpectedPhase: v1alpha1.PhaseDeprovisioned,
			FakeClient:    fake.NewSimpleClientset(),
			FakeSDK: &keycloak.SdkCruderMock{
				UpdateFunc: func(object sdk.Object) error {
					return nil
				},
			},
			FakeKCF: &keycloak.KeycloakClientFactoryMock{},
			FakeHandler: &HandlerMock{
				DeprovisionFunc: func(realm *v1alpha1.KeycloakRealm) (*v1alpha1.KeycloakRealm, error) {
					return realm, nil
				},
				PreflightChecksFunc: func(realm *v1alpha1.KeycloakRealm) (*v1alpha1.KeycloakRealm, error) {
					return realm, nil
				},
			},
			ExpectedError: "",
		},
		{
			Name:    "No error when deletion timestamp is set and phase is deprovisioned",
			Context: context.TODO(),
			Object: &v1alpha1.KeycloakRealm{
				Status: v1alpha1.KeycloakRealmStatus{
					Phase: v1alpha1.PhaseDeprovisioned,
				},
				ObjectMeta: metav1.ObjectMeta{
					DeletionTimestamp: &now,
				},
			},
			ExpectedPhase: v1alpha1.PhaseComplete,
			FakeClient:    fake.NewSimpleClientset(),
			FakeSDK: &keycloak.SdkCruderMock{
				UpdateFunc: func(object sdk.Object) error {
					return nil
				},
			},
			FakeKCF: &keycloak.KeycloakClientFactoryMock{},
			FakeHandler: &HandlerMock{
				PreflightChecksFunc: func(realm *v1alpha1.KeycloakRealm) (*v1alpha1.KeycloakRealm, error) {
					return realm, nil
				},
			},
			ExpectedError: "",
		},
		{
			Name:    "No error when no phase is set",
			Context: context.TODO(),
			Object: &v1alpha1.KeycloakRealm{
				Status: v1alpha1.KeycloakRealmStatus{
					Phase: v1alpha1.NoPhase,
				},
			},
			ExpectedPhase: v1alpha1.PhaseAccepted,
			FakeClient:    fake.NewSimpleClientset(),
			FakeSDK: &keycloak.SdkCruderMock{
				UpdateFunc: func(object sdk.Object) error {
					return nil
				},
			},
			FakeKCF: &keycloak.KeycloakClientFactoryMock{},
			FakeHandler: &HandlerMock{
				InitialiseFunc: func(kcr *v1alpha1.KeycloakRealm) (*v1alpha1.KeycloakRealm, error) {
					kcr.Status.Phase = v1alpha1.PhaseAccepted
					return kcr, nil
				},
				PreflightChecksFunc: func(realm *v1alpha1.KeycloakRealm) (*v1alpha1.KeycloakRealm, error) {
					return realm, nil
				},
			},
			ExpectedError: "",
		},
		{
			Name:    "No error when phase is accepted",
			Context: context.TODO(),
			Object: &v1alpha1.KeycloakRealm{
				Status: v1alpha1.KeycloakRealmStatus{
					Phase: v1alpha1.PhaseAccepted,
				},
			},
			ExpectedPhase: v1alpha1.PhaseProvision,
			FakeClient:    fake.NewSimpleClientset(),
			FakeSDK: &keycloak.SdkCruderMock{
				UpdateFunc: func(object sdk.Object) error {
					return nil
				},
			},
			FakeKCF: &keycloak.KeycloakClientFactoryMock{},
			FakeHandler: &HandlerMock{
				AcceptedFunc: func(kcr *v1alpha1.KeycloakRealm) (*v1alpha1.KeycloakRealm, error) {
					kcr.Status.Phase = v1alpha1.PhaseProvision
					return kcr, nil
				},
				PreflightChecksFunc: func(realm *v1alpha1.KeycloakRealm) (*v1alpha1.KeycloakRealm, error) {
					return realm, nil
				},
			},
			ExpectedError: "",
		},
		{
			Name:    "No error when phase is provision",
			Context: context.TODO(),
			Object: &v1alpha1.KeycloakRealm{
				Status: v1alpha1.KeycloakRealmStatus{
					Phase: v1alpha1.PhaseProvision,
				},
			},
			ExpectedPhase: v1alpha1.PhaseReconcile,
			FakeClient:    fake.NewSimpleClientset(),
			FakeSDK: &keycloak.SdkCruderMock{
				UpdateFunc: func(object sdk.Object) error {
					return nil
				},
			},
			FakeKCF: &keycloak.KeycloakClientFactoryMock{},
			FakeHandler: &HandlerMock{
				ProvisionFunc: func(kcr *v1alpha1.KeycloakRealm) (*v1alpha1.KeycloakRealm, error) {
					kcr.Status.Phase = v1alpha1.PhaseReconcile
					return kcr, nil
				},
				PreflightChecksFunc: func(realm *v1alpha1.KeycloakRealm) (*v1alpha1.KeycloakRealm, error) {
					return realm, nil
				},
			},
			ExpectedError: "",
		},
		{
			Name:    "No error when phase is provisioned",
			Context: context.TODO(),
			Object: &v1alpha1.KeycloakRealm{
				Status: v1alpha1.KeycloakRealmStatus{
					Phase: v1alpha1.PhaseReconcile,
				},
			},
			ExpectedPhase: v1alpha1.PhaseReconcile,
			FakeClient:    fake.NewSimpleClientset(),
			FakeSDK: &keycloak.SdkCruderMock{
				UpdateFunc: func(object sdk.Object) error {
					return nil
				},
			},
			FakeKCF: &keycloak.KeycloakClientFactoryMock{},
			FakeHandler: &HandlerMock{
				ReconcileFunc: func(kcr *v1alpha1.KeycloakRealm) (*v1alpha1.KeycloakRealm, error) {
					return kcr, nil
				},
				PreflightChecksFunc: func(realm *v1alpha1.KeycloakRealm) (*v1alpha1.KeycloakRealm, error) {
					return realm, nil
				},
			},
			ExpectedError: "",
		},
		{
			Name:    "Phase not altered when handler returns an error",
			Context: context.TODO(),
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
			},
			FakeKCF: &keycloak.KeycloakClientFactoryMock{},
			FakeHandler: &HandlerMock{
				AcceptedFunc: func(kcr *v1alpha1.KeycloakRealm) (*v1alpha1.KeycloakRealm, error) {
					return kcr, errors.New("Error yo")
				},
				PreflightChecksFunc: func(realm *v1alpha1.KeycloakRealm) (*v1alpha1.KeycloakRealm, error) {
					return realm, nil
				},
			},
			ExpectedError: "Error yo",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.Name, func(t *testing.T) {
			rh := NewRealmHandler(testCase.FakeKCF, testCase.FakeSDK, testCase.FakeHandler)
			err := rh.Handle(testCase.Context, testCase.Object, testCase.Deleted)
			if testCase.Object.Status.Phase != testCase.ExpectedPhase {
				t.Fatalf("expected phase: '%v', got '%v'", testCase.ExpectedPhase, testCase.Object.Status.Phase)
			}
			if testCase.ExpectedError == "" && err != nil {
				t.Fatalf("unexpected error: '%v'", err.Error())
			}
			if testCase.ExpectedError != "" && err == nil {
				t.Fatalf("expected error containing: '%v', got nil", testCase.ExpectedError)
			}
			if testCase.ExpectedError != "" && err != nil {
				if !strings.Contains(err.Error(), testCase.ExpectedError) {
					t.Fatalf("Expected error containing '%v', got '%v'", testCase.ExpectedError, err.Error())
				}
			}
		})
	}
}
func TestGVK(t *testing.T) {
	testee := NewRealmHandler(&keycloak.KeycloakClientFactoryMock{}, &keycloak.SdkCruderMock{}, &HandlerMock{})
	gvk := testee.GVK()

	if gvk.Group != v1alpha1.Group {
		t.Fatalf("expected group: %v, got %v", v1alpha1.Group, gvk.Group)
	}
	if gvk.Version != v1alpha1.Version {
		t.Fatalf("expected version: %v, got %v", v1alpha1.Version, gvk.Version)
	}
	if gvk.Kind != v1alpha1.KeycloakRealmKind {
		t.Fatalf("expected kind: %v, got %v", v1alpha1.KeycloakRealmKind, gvk.Kind)
	}
}
