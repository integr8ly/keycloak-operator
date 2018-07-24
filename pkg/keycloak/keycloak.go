package keycloak

import (
	"context"
	"fmt"

	"sync"

	"github.com/aerogear/keycloak-operator/pkg/apis/aerogear/v1alpha1"
	"github.com/operator-framework/operator-sdk/pkg/sdk"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type Handler struct {
	kcClientFactory KeycloakClientFactory
}

func NewHandler(kcClientFactory KeycloakClientFactory) *Handler {
	return &Handler{
		kcClientFactory: kcClientFactory,
	}
}

func (h *Handler) Handle(ctx context.Context, event sdk.Event) error {
	fmt.Println("handling object ", event.Object.GetObjectKind().GroupVersionKind().String())
	// first reconcile if the phase is empty set the phase to accepted
	kc := event.Object.(*v1alpha1.Keycloak)
	kcCopy := kc.DeepCopy()
	// set up authenticated client
	authenticatedClient, err := h.kcClientFactory.AuthenticatedClient(*kcCopy)
	if err != nil {
		return errors.Wrap(err, "failed to get authenticated client for keycloak")
	}
	if event.Deleted {
		return h.deleteKeycloak(kcCopy)
	}
	if kc.Status.Phase == v1alpha1.PhaseAccepted {
		logrus.Info("not doing anything as this resource is already being worked on")
		return nil
	}
	//some validation and setting of defaults should be done here in the future before accepting
	kcCopy.Status.Phase = v1alpha1.PhaseAccepted
	kcCopy.Status.Ready = false
	if err := sdk.Update(kcCopy); err != nil {
		return errors.Wrap(err, "failed to update the keycloak resource")
	}

	// hand of each realm to reconcile realm may want to make async to avoid blocking
	for _, r := range kcCopy.Spec.Realms {
		if err := h.reconcileRealm(ctx, r, authenticatedClient); err != nil {
			return errors.Wrap(err, "failed to reconcile realm "+r.Name)
		}
	}

	return nil
}

func (h *Handler) reconcileRealm(ctx context.Context, realm v1alpha1.KeycloakRealm, authenticatedClient KeycloakInterface) error {
	rc := realm.DeepCopy()
	// check does realm exist
	exists, err := authenticatedClient.DoesRealmExist(rc.Name)
	if err != nil {
		return err
	}
	if !exists {
		// create realm
	}

	return nil
}

func (h *Handler) reconcileClient(ctx context.Context, wg *sync.WaitGroup, clientDef v1alpha1.KeycloakClient, authenticatedClient KeycloakInterface) error {
	return nil
}

func (h *Handler) reconcileUser(ctx context.Context, wg *sync.WaitGroup, userDef v1alpha1.KeycloakUser, authenticatedClient KeycloakInterface) error {
	return nil
}

func (h *Handler) deleteKeycloak(kc *v1alpha1.Keycloak) error {
	return nil
}

func (h *Handler) GVK() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Version: v1alpha1.Version,
		Group:   v1alpha1.Group,
		Kind:    v1alpha1.KeycloakKind,
	}
}
