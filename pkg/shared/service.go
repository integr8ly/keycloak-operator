package shared

import (
	"context"
	"github.com/aerogear/keycloak-operator/pkg/apis/aerogear/v1alpha1"
	"github.com/operator-framework/operator-sdk/pkg/sdk"
	"k8s.io/apimachinery/pkg/runtime/schema"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
	"github.com/sirupsen/logrus"
	sc "github.com/kubernetes-incubator/service-catalog/pkg/api/meta"
)

type ServiceHandler struct {
}

func NewServiceHandler() *ServiceHandler {
	return &ServiceHandler{}
}

func (sh *ServiceHandler) Handle(ctx context.Context, event sdk.Event) error {
	logrus.Debug("handling object ", event.Object.GetObjectKind().GroupVersionKind().String())

	sharedService := event.Object.(*v1alpha1.SharedService)
	sharedServiceCopy := sharedService.DeepCopy()

	if sharedService.Spec.ServiceType != strings.ToLower(v1alpha1.KeycloakKind) {
		return nil
	}

	if event.Deleted {
		return nil
	}

	if sharedService.GetDeletionTimestamp() != nil {
		return sh.finalizeSharedService(sharedServiceCopy)
	}

	logrus.Debugf("SharedServicePhase: %v", sharedService.Status.Phase)
	switch sharedService.Status.Phase {
	case v1alpha1.SSPhaseNone:
		sh.initSharedService(sharedServiceCopy)
	case v1alpha1.SSPhaseAccepted:
		sh.createKeycloaks(sharedServiceCopy)
	}

	return nil
}

func (sh *ServiceHandler) createKeycloaks(sharedService *v1alpha1.SharedService) error {
	keycloakList := v1alpha1.KeycloakList{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Keycloak",
			APIVersion: "aerogear.org/v1alpha1",
		},
	}
	listOptions := sdk.WithListOptions(&metav1.ListOptions{
		LabelSelector:        "aerogear.org/sharedServiceName=" + sharedService.ObjectMeta.Name,
		IncludeUninitialized: false,
	})

	err := sdk.List(sharedService.Namespace, &keycloakList, listOptions)
	if err != nil {
		logrus.Errorf("Failed to query keycloaks : %v", err)
		return err
	}

	numCurrentInstances := len(keycloakList.Items)
	numRequiredInstances := minRequiredInstances(sharedService.Spec.MinInstances, sharedService.Spec.RequiredInstances)
	numMaxInstances := sharedService.Spec.MaxInstances
	logrus.Debugf("number of service instances(%v), current: %v, required: %v, max: %v", sharedService.Spec.ServiceType, numCurrentInstances, numRequiredInstances, numMaxInstances)

	if numCurrentInstances < numRequiredInstances && numCurrentInstances < numMaxInstances {
		err := sdk.Create(newKeycloak(sharedService))
		if err != nil {
			logrus.Errorf("Failed to create keycloak : %v", err)
			return err
		}
	} else {
		sharedService.Status.Ready = true
		sharedService.Status.Phase = v1alpha1.SSPhaseComplete
		err := sdk.Update(sharedService)
		if err != nil {
			logrus.Errorf("error updating resource status: %v", err)
			return err
		}
	}

	return nil
}

func minRequiredInstances(minInstances, requiredInstances int) int {
	if minInstances > requiredInstances {
		return minInstances
	}
	return requiredInstances
}

func newKeycloak(sharedService *v1alpha1.SharedService) *v1alpha1.Keycloak {
	labels := map[string]string{
		"aerogear.org/sharedServiceName": sharedService.ObjectMeta.Name,
	}
	return &v1alpha1.Keycloak{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Keycloak",
			APIVersion: "aerogear.org/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: sharedService.Name + "-keycloak-",
			Namespace:    sharedService.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(sharedService, schema.GroupVersionKind{
					Group:   v1alpha1.SchemeGroupVersion.Group,
					Version: v1alpha1.SchemeGroupVersion.Version,
					Kind:    "SharedService",
				}),
			},
			Finalizers: []string{v1alpha1.KeycloakFinalizer},
			Labels: labels,
		},
		Spec: v1alpha1.KeycloakSpec{
			Version:          v1alpha1.KeycloakVersion,
			AdminCredentials: "",
			Realms:           []v1alpha1.KeycloakRealm{},
		},
		Status: v1alpha1.KeycloakStatus{
			SharedConfig: v1alpha1.StatusSharedConfig{
				MaxSlices:     sharedService.Spec.MaxSlices,
				CurrentSlices: 0,
			},
		},
	}
}

func (sh *ServiceHandler) initSharedService(sharedService *v1alpha1.SharedService) error {
	logrus.Infof("initialise shared service: %v", sharedService)
	sc.AddFinalizer(sharedService, v1alpha1.SharedServiceFinalizer)
	sharedService.Status.Phase = v1alpha1.SSPhaseAccepted
	err := sdk.Update(sharedService)
	if err != nil {
		logrus.Errorf("error updating resource finalizer: %v", err)
		return err
	}
	return nil
}

func (sh *ServiceHandler) finalizeSharedService(sharedService *v1alpha1.SharedService) error {
	logrus.Infof("finalise shared service: %v", sharedService)
	sc.RemoveFinalizer(sharedService, v1alpha1.SharedServiceFinalizer)
	err := sdk.Update(sharedService)
	if err != nil {
		logrus.Errorf("error updating resource finalizer: %v", err)
		return err
	}
	return nil
}

func (sh *ServiceHandler) GVK() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Kind:    v1alpha1.SharedServiceKind,
		Group:   v1alpha1.Group,
		Version: v1alpha1.Version,
	}
}
