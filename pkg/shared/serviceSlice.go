package shared

import (
	"context"

	"github.com/aerogear/keycloak-operator/pkg/apis/aerogear/v1alpha1"
	"github.com/operator-framework/operator-sdk/pkg/sdk"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ServiceSliceHandler struct {
}

func NewServiceSliceHandler() *ServiceSliceHandler {
	return &ServiceSliceHandler{}
}

func (sh *ServiceSliceHandler) Handle(ctx context.Context, event sdk.Event) error {
	return nil
}

func (sh *ServiceSliceHandler) GVK() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Kind:    v1alpha1.SharedServiceSliceKind,
		Group:   v1alpha1.Group,
		Version: v1alpha1.Version,
	}
}
