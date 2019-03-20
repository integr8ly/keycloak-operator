package realm

import (
	"context"

	"github.com/integr8ly/keycloak-operator/pkg/apis/aerogear/v1alpha1"
	"github.com/integr8ly/keycloak-operator/pkg/keycloak"
	"github.com/pkg/errors"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

//go:generate moq -out handler_moq.go . Handler

type Handler interface {
	Initialise(realm *v1alpha1.KeycloakRealm) (*v1alpha1.KeycloakRealm, error)
	Accepted(realm *v1alpha1.KeycloakRealm) (*v1alpha1.KeycloakRealm, error)
	Provision(realm *v1alpha1.KeycloakRealm) (*v1alpha1.KeycloakRealm, error)
	Reconcile(realm *v1alpha1.KeycloakRealm) (*v1alpha1.KeycloakRealm, error)
	Deprovision(realm *v1alpha1.KeycloakRealm) (*v1alpha1.KeycloakRealm, error)
	PreflightChecks(realm *v1alpha1.KeycloakRealm) (*v1alpha1.KeycloakRealm, error)
}

func NewRealmHandler(kcClientFactory keycloak.KeycloakClientFactory, cruder keycloak.SdkCruder, handler Handler) *realmHandler {
	return &realmHandler{
		kcClientFactory: kcClientFactory,
		sdkCrud:         cruder,
		handler:         handler,
	}
}

type realmHandler struct {
	kcClientFactory keycloak.KeycloakClientFactory
	sdkCrud         keycloak.SdkCruder
	handler         Handler
}

func (r *realmHandler) handleDelete(kcr *v1alpha1.KeycloakRealm) error {
	switch kcr.Status.Phase {
	case v1alpha1.PhaseInstanceDeprovisioned:
		kcr.Finalizers = []string{}
		kcr.Status.Phase = v1alpha1.PhaseComplete
		return r.sdkCrud.Update(kcr)
	case v1alpha1.PhaseDeprovisioned:
		kcr.Finalizers = []string{}
		kcr.Status.Phase = v1alpha1.PhaseComplete
		return r.sdkCrud.Update(kcr)
	default:
		_, err := r.handler.Deprovision(kcr)
		if err != nil {
			kcr.Status.Phase = v1alpha1.PhaseDeprovisionFailed
			return err
		}
		kcr.Status.Phase = v1alpha1.PhaseDeprovisioned
		return r.sdkCrud.Update(kcr)
	}
}

func (r *realmHandler) Handle(context context.Context, object interface{}, deleted bool) error {
	if deleted {
		return nil
	}

	kcr, ok := object.(*v1alpha1.KeycloakRealm)
	if !ok {
		return errors.New("error converting object to keycloak realm")
	}

	if _, err := r.handler.PreflightChecks(kcr); err != nil {
		//cannot contact keycloak API
		kcr.Status.Message = errors.Wrap(err, "failed reconciliation").Error()
		return r.sdkCrud.Update(kcr)
	}
	kcr.Status.Message = ""
	if kcr.GetDeletionTimestamp() != nil {
		return r.handleDelete(kcr)
	}
	switch kcr.Status.Phase {
	case v1alpha1.NoPhase:
		_, err := r.handler.Initialise(kcr)
		if err != nil {
			return err
		}
		return r.sdkCrud.Update(kcr)
	case v1alpha1.PhaseAccepted:
		_, err := r.handler.Accepted(kcr)
		if err != nil {
			return err
		}
		return r.sdkCrud.Update(kcr)
	case v1alpha1.PhaseProvision:
		kcr, err := r.handler.Provision(kcr)
		if err != nil {
			return err
		}
		return r.sdkCrud.Update(kcr)
	case v1alpha1.PhaseReconcile:
		_, err := r.handler.Reconcile(kcr)
		if err != nil {
			return err
		}
		return r.sdkCrud.Update(kcr)
	}

	return nil
}

func (r *realmHandler) GVK() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Version: v1alpha1.Version,
		Group:   v1alpha1.Group,
		Kind:    v1alpha1.KeycloakRealmKind,
	}
}
