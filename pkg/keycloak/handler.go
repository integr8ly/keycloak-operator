package keycloak

import (
	"context"

	"github.com/aerogear/keycloak-operator/pkg/apis/areogear/v1alpha1"

	"fmt"

	"github.com/operator-framework/operator-sdk/pkg/sdk"
	"github.com/sirupsen/logrus"
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
	case *v1alpha1.Keycloak:
		fmt.Println("keycloak", o.Name, o.Namespace)
		if event.Deleted {
			return h.keycloakDeleted(ctx, o)
		}
		return h.keycloakAddedUpdated(ctx, o)
	}
	return nil
}

func (h *Handler) keycloakAddedUpdated(ctx context.Context, kc *v1alpha1.Keycloak) error {
	logrus.Info("keycloak added updated ", kc)
	return nil
}

func (h *Handler) keycloakDeleted(ctx context.Context, kc *v1alpha1.Keycloak) error {
	logrus.Info("keycloak deleted ", kc)
	return nil
}
