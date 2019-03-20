package dispatch

import (
	"context"

	"github.com/operator-framework/operator-sdk/pkg/sdk"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
)

func NewHandler(k8Client kubernetes.Interface) sdk.Handler {
	return &Handler{
		k8Client:    k8Client,
		gvkHandlers: map[schema.GroupVersionKind]MuxHandler{},
	}
}

type Handler struct {
	// Fill me
	k8Client    kubernetes.Interface
	gvkHandlers map[schema.GroupVersionKind]MuxHandler
}

func (h *Handler) Handle(ctx context.Context, event sdk.Event) error {
	if handler, ok := h.gvkHandlers[event.Object.GetObjectKind().GroupVersionKind()]; ok {
		return handler.Handle(ctx, event.Object, event.Deleted)
	}
	return errors.New("no handler registered for group version kind " + event.Object.GetObjectKind().GroupVersionKind().String())
}

type MuxHandler interface {
	Handle(context.Context, interface{}, bool) error
	GVK() schema.GroupVersionKind
}

func (h *Handler) AddHandler(handler MuxHandler) {
	h.gvkHandlers[handler.GVK()] = handler
}
