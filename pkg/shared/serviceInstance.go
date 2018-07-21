package shared

import (
	"context"
	"fmt"

	"github.com/aerogear/keycloak-operator/pkg/apis/aerogear/v1alpha1"
	"github.com/operator-framework/operator-sdk/pkg/sdk"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ServiceInstanceHandler struct {
}

func NewServiceInstanceHandler() *ServiceInstanceHandler {
	return &ServiceInstanceHandler{}
}

func (sh *ServiceInstanceHandler) Handle(ctx context.Context, event sdk.Event) error {
	fmt.Println("handling object ", event.Object.GetObjectKind().GroupVersionKind().String())
	return nil
}

func (sh *ServiceInstanceHandler) GVK() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Kind:    v1alpha1.SharedServiceInstanceKind,
		Group:   v1alpha1.Group,
		Version: v1alpha1.Version,
	}
}
