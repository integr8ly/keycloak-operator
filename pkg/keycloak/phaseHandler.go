package keycloak

import (
	"fmt"

	"strings"

	"github.com/aerogear/keycloak-operator/pkg/apis/aerogear/v1alpha1"
	v14 "github.com/openshift/client-go/apps/clientset/versioned/typed/apps/v1"
	v13 "github.com/openshift/client-go/route/clientset/versioned/typed/route/v1"
	"github.com/operator-framework/operator-sdk/pkg/util/k8sutil"
	"github.com/pkg/errors"
	"k8s.io/api/core/v1"
	errors2 "k8s.io/apimachinery/pkg/api/errors"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

type phaseHandler struct {
	k8sClient                    kubernetes.Interface
	dynamicResourceClientFactory func(apiVersion, kind, namespace string) (dynamic.ResourceInterface, string, error)
	ocRouteClient                v13.RouteV1Interface
	ocDCClient                   v14.AppsV1Interface
}

func NewPhaseHandler(k8sClient kubernetes.Interface, ocRouteClient v13.RouteV1Interface, ocDCClient v14.AppsV1Interface, dynamicResourceClientFactory func(apiVersion, kind, namespace string) (dynamic.ResourceInterface, string, error)) *phaseHandler {
	return &phaseHandler{
		k8sClient:                    k8sClient,
		dynamicResourceClientFactory: dynamicResourceClientFactory,
		ocRouteClient:                ocRouteClient,
		ocDCClient:                   ocDCClient,
	}
}

func (ph *phaseHandler) Initialise(sso *v1alpha1.Keycloak) (*v1alpha1.Keycloak, error) {
	// copy state and modify return state
	kcState := sso.DeepCopy()
	// fill in any defaults that are not set
	kcState.Defaults()
	// validate
	if err := kcState.Validate(); err != nil {
		return nil, errors.Wrap(err, "validation failed")
	}

	// set the finalizer
	if err := v1alpha1.AddFinalizer(kcState, v1alpha1.KeycloakFinalizer); err != nil {
		return nil, err
	}
	// set the phase to accepted or set a message that it cannot be accepted
	kcState.Status.Phase = v1alpha1.PhaseAccepted
	return kcState, nil
}

func (ph *phaseHandler) Accepted(sso *v1alpha1.Keycloak) (*v1alpha1.Keycloak, error) {

	kc := sso.DeepCopy()
	if kc.Spec.AdminCredentials != "" {
		return nil, nil
	}

	adminPwd, err := GeneratePassword()
	if err != nil {
		return nil, err
	}
	namespace := kc.ObjectMeta.Namespace
	data := map[string][]byte{"SSO_ADMIN_USERNAME": []byte("admin"), "SSO_ADMIN_PASSWORD": []byte(adminPwd)}
	adminCredentialsSecret := &v1.Secret{
		TypeMeta: v12.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: v12.ObjectMeta{
			Labels:    map[string]string{"application": "sso", "sso": kc.Name},
			Namespace: namespace,
			Name:      "credential-" + kc.Name,
		},
		Data: data,
		Type: "Opaque",
	}
	adminCredential, err := ph.k8sClient.CoreV1().Secrets(namespace).Create(adminCredentialsSecret)
	if err != nil && !errors2.IsAlreadyExists(err) {
		return nil, err
	}

	kc.Spec.AdminCredentials = adminCredential.GetName()
	kc.Status.Phase = v1alpha1.PhaseProvision
	return kc, nil
}

func (ph *phaseHandler) Provision(sso *v1alpha1.Keycloak) (*v1alpha1.Keycloak, error) {
	// copy state and modify return state
	kc := sso.DeepCopy()
	adminCreds, err := ph.k8sClient.CoreV1().Secrets(kc.Namespace).Get(kc.Spec.AdminCredentials, v12.GetOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the secret for the admin credentials")
	}
	decodedParams := map[string]string{}
	for k, v := range adminCreds.Data {
		decodedParams[k] = string(v)
	}
	objects, err := GetInstallResourcesAsRuntimeObjects(kc, decodedParams)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get runtime objects during provision")
	}
	for _, o := range objects {
		gvk := o.GetObjectKind().GroupVersionKind()
		apiVersion, kind := gvk.ToAPIVersionAndKind()
		resourceClient, _, err := ph.dynamicResourceClientFactory(apiVersion, kind, kc.Namespace)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("failed to get resource client: %v", err))
		}

		unstructObj, err := k8sutil.UnstructuredFromRuntimeObject(o)
		if err != nil {
			return nil, errors.Wrap(err, "failed to turn runtime object "+o.GetObjectKind().GroupVersionKind().String()+" into unstructured object during provision")
		}
		unstructObj, err = resourceClient.Create(unstructObj)
		if err != nil && !errors2.IsAlreadyExists(err) {
			return nil, errors.Wrap(err, "failed to create object during provision with kind "+o.GetObjectKind().GroupVersionKind().String())
		}
	}

	kc.Status.Phase = v1alpha1.PhaseProvisioned
	return kc, nil
}

func (ph *phaseHandler) Provisioned(sso *v1alpha1.Keycloak) (*v1alpha1.Keycloak, error) {
	kc := sso.DeepCopy()
	podList, err := ph.k8sClient.CoreV1().Pods(kc.Namespace).List(v12.ListOptions{
		LabelSelector:        fmt.Sprintf("application=%v", SSO_APPLICATION_NAME),
		IncludeUninitialized: false,
	})

	if err != nil || len(podList.Items) == 0 {
		return kc, nil
	}
	for _, pod := range podList.Items {
		for _, condition := range pod.Status.Conditions {
			if condition.Type == "Ready" && condition.Status != "True" {
				return kc, nil
			}
		}
	}
	routeList, _ := ph.ocRouteClient.Routes(kc.Namespace).List(v12.ListOptions{LabelSelector: "application=sso"})
	for _, route := range routeList.Items {
		if route.Spec.To.Name == SSO_ROUTE_NAME {
			protocol := "https"
			if route.Spec.TLS == nil {
				protocol = "http"
			}
			url := fmt.Sprintf("%v://%v", protocol, route.Spec.Host)
			secret, err := ph.k8sClient.CoreV1().Secrets(kc.Namespace).Get(kc.Spec.AdminCredentials, v12.GetOptions{})
			secret.Data["SSO_ADMIN_URL"] = []byte(url)
			if _, err = ph.k8sClient.CoreV1().Secrets(kc.Namespace).Update(secret); err != nil {
				return nil, errors.Wrap(err, "could not update admin credentials")
			}
		}
	}
	kc.Status.Phase = v1alpha1.PhaseComplete
	return kc, nil
}

func (ph *phaseHandler) Deprovision(sso *v1alpha1.Keycloak) (*v1alpha1.Keycloak, error) {
	kc := sso.DeepCopy()
	if _, err := v1alpha1.RemoveFinalizer(kc, v1alpha1.KeycloakFinalizer); err != nil {
		return nil, errors.Wrap(err, "failed to remove finalizer for "+kc.Name)
	}
	namespace := kc.ObjectMeta.Namespace
	deleteOpts := v12.NewDeleteOptions(0)
	listOpts := v12.ListOptions{LabelSelector: "application=sso"}
	// delete dcs
	if err := ph.ocDCClient.DeploymentConfigs(namespace).DeleteCollection(deleteOpts, listOpts); err != nil {
		return nil, errors.Wrap(err, "failed to remove the deployment configs")
	}
	// delete pvc
	if err := ph.k8sClient.CoreV1().PersistentVolumeClaims(kc.Namespace).DeleteCollection(deleteOpts, listOpts); err != nil {
		return nil, errors.Wrap(err, "failed to remove the pvc")
	}
	// delete routes
	if err := ph.ocRouteClient.Routes(kc.Namespace).DeleteCollection(deleteOpts, listOpts); err != nil {
		return nil, errors.Wrap(err, "failed to remove the routes")
	}

	// delete secrets
	if err := ph.k8sClient.CoreV1().Secrets(kc.Namespace).DeleteCollection(deleteOpts, listOpts); err != nil {
		return nil, errors.Wrap(err, "failed to remove the secrets")
	}
	// delete services
	services, err := ph.k8sClient.CoreV1().Services(kc.Namespace).List(listOpts)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list all services for sso")
	}
	// todo handle more than one error
	var errs []string
	for _, s := range services.Items {
		if err = ph.k8sClient.CoreV1().Services(kc.Namespace).Delete(s.Name, deleteOpts); err != nil {
			errs = append(errs, err.Error())
		}

	}
	if len(errs) > 0 {
		errMsg := strings.Join(errs[:], " : ")
		return nil, errors.New("failed to remove services while deprovisioning " + errMsg)
	}
	return kc, nil
}
