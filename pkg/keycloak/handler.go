package keycloak

import (
	"context"

	"github.com/aerogear/keycloak-operator/pkg/apis/areogear/v1alpha1"

	"fmt"

	"github.com/operator-framework/operator-sdk/pkg/sdk"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func NewHandler(k8Client kubernetes.Interface) sdk.Handler {
	return &Handler{
		k8Client: k8Client,
	}
}

type Handler struct {
	// Fill me
	k8Client kubernetes.Interface
}

func (h *Handler) Handle(ctx context.Context, event sdk.Event) error {
	switch o := event.Object.(type) {
	case *v1alpha1.KeycloakRealm:
		fmt.Println("keycloak realm ", o.Name, o.Namespace)
		if event.Deleted {
			return h.keycloakRealmDeleted(ctx, o)
		}
		return h.keycloakRealmAddedUpdated(ctx, o)
	case *v1alpha1.KeycloakClientSync:
		fmt.Println("keycloak client sync ", o.Name, o.Namespace)
		if event.Deleted {
			return h.keycloakClientSyncDeleted(ctx, o)
		}
		return h.keycloakClientSyncAddedUpdated(ctx, o)
	case *v1alpha1.Keycloak:
		fmt.Println("keycloak ", o.Name, o.Namespace)
		if event.Deleted {
			return h.keycloakDeleted(ctx, o)
		}
		return h.keycloakAddedUpdated(ctx, o)

	case *v1alpha1.KeycloakClient:
		fmt.Println("keycloak client ", o.Name, o.Namespace)
		if event.Deleted {
			return h.keycloakClientDeleted(ctx, o)
		}
		return h.keycloakClientAddedUpdated(ctx, o)
	}
	return nil
}

func (h *Handler) keycloakAddedUpdated(ctx context.Context, keycloak *v1alpha1.Keycloak) error {
	kcCopy := keycloak.DeepCopy()
	if kcCopy.Status.Phase == "" {
		//first pass set the phase to accepted
		kcCopy.Status.Phase = v1alpha1.PhaseAccepted
		//TODO create the client and pass in as interface to allow testing
		return sdk.Update(kcCopy)
	}
	//once the resource is in accepted phase only certain things can happen to it. We save it to a phase first as we are in a sync loop and want
	// to avoid working on the same resource in different threads (need to investigate to the sdk or k8s client do anything to avoid this)
	if (kcCopy.Status.Phase == v1alpha1.PhaseAccepted) || (kcCopy.Status.Phase == v1alpha1.PhaseAuthFailed) {
		_, token, err := NewAuthenticatedClientForKeycloak(*kcCopy, h.k8Client)
		if err != nil {
			return err
		}
		kcCopy.Status.Phase = v1alpha1.PhaseComplete
		kcCopy.Spec.Token = token
		return sdk.Update(kcCopy)
	}
	return nil
}

func (h *Handler) keycloakDeleted(ctx context.Context, keycloak *v1alpha1.Keycloak) error {
	/// may not need to do anything here
	fmt.Println("keycloak deleted")
	return nil
}

func (h *Handler) keycloakRealmAddedUpdated(ctx context.Context, realm *v1alpha1.KeycloakRealm) error {
	realmCopy := realm.DeepCopy()
	if realmCopy.Status.Phase == "" {
		realmCopy.Status.Phase = v1alpha1.PhaseAccepted
		return sdk.Update(realmCopy)
	}
	if realmCopy.Status.Phase == v1alpha1.PhaseAccepted {
		kc := &v1alpha1.Keycloak{
			ObjectMeta: metav1.ObjectMeta{
				Name:      realmCopy.Spec.KeycloakID,
				Namespace: realmCopy.Namespace,
			}}
		getOpts := sdk.WithGetOptions(&metav1.GetOptions{})
		if err := sdk.Get(kc, getOpts); err != nil {
			return errors.Wrap(err, "failed to get keycloak")
		}
		kcClient, err := NewAuthenticatedClientForKeycloak(*kc, h.k8Client)
		if err != nil {
			return errors.Wrap(err, "")
		}
		return kcClient.CreateRealm()
	}
	return nil
}

func (h *Handler) keycloakRealmDeleted(ctx context.Context, realm *v1alpha1.KeycloakRealm) error {
	return nil
}

func (h *Handler) keycloakClientSyncAddedUpdated(ctx context.Context, realm *v1alpha1.KeycloakClientSync) error {
	return nil
}

func (h *Handler) keycloakClientSyncDeleted(ctx context.Context, realm *v1alpha1.KeycloakClientSync) error {
	return nil
}

func (h *Handler) keycloakClientDeleted(ctx context.Context, realm *v1alpha1.KeycloakClient) error {
	return nil
}

func (h *Handler) keycloakClientAddedUpdated(ctx context.Context, realm *v1alpha1.KeycloakClient) error {
	return nil
}
