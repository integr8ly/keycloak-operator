package shared

import (
	"context"

	"github.com/aerogear/keycloak-operator/pkg/apis/aerogear/v1alpha1"
	"github.com/operator-framework/operator-sdk/pkg/sdk"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ServiceHandler struct {
}

func NewServiceHandler() *ServiceHandler {
	return &ServiceHandler{}
}

func (sh *ServiceHandler) Handle(ctx context.Context, event sdk.Event) error {
	return nil
}

func (sh *ServiceHandler) GVK() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Kind:    v1alpha1.SharedServiceKind,
		Group:   v1alpha1.Group,
		Version: v1alpha1.Version,
	}
}
