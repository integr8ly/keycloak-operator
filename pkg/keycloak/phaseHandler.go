package keycloak

import (
	"fmt"
	"github.com/integr8ly/keycloak-operator/pkg/util"
	"github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/api/batch/v1beta1"
	"strings"

	"github.com/integr8ly/keycloak-operator/pkg/apis/aerogear/v1alpha1"
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
	kcState.Status.Version = SSO_VERSION
	return kcState, nil
}

func (ph *phaseHandler) Accepted(sso *v1alpha1.Keycloak) (*v1alpha1.Keycloak, error) {
	var err error
	kc := sso.DeepCopy()
	adminPwd := kc.Spec.AdminCredentials

	if adminPwd == "" {
		adminPwd, err = GeneratePassword()
		if err != nil {
			return sso, err
		}
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
		return sso, err
	}

	kc.Spec.AdminCredentials = adminCredential.GetName()
	kc.Status.Phase = v1alpha1.PhaseAwaitProvision
	return kc, nil
}

func (ph *phaseHandler) Provision(sso *v1alpha1.Keycloak) (*v1alpha1.Keycloak, error) {
	// copy state and modify return state
	kc := sso.DeepCopy()
	secretName := "credential-" + kc.Name

	adminCreds, err := ph.k8sClient.CoreV1().Secrets(kc.Namespace).Get(secretName, v12.GetOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the secret for the admin credentials")
	}

	// List of plugins passed in the custom resource
	plugins := sso.Spec.Plugins
	decodedParams := map[string]string{
		"SSO_PLUGINS": strings.Join(plugins, ","),
	}

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

	kc.Status.Phase = v1alpha1.PhaseWaitForPodsToRun
	return kc, nil
}

func (ph *phaseHandler) WaitforPods(sso *v1alpha1.Keycloak) (*v1alpha1.Keycloak, error) {
	kc := sso.DeepCopy()
	podList, err := ph.k8sClient.CoreV1().Pods(kc.Namespace).List(v12.ListOptions{
		LabelSelector:        fmt.Sprintf("application=%v", SSO_APPLICATION_NAME),
		IncludeUninitialized: false,
	})

	if err != nil || len(podList.Items) == 0 {
		return kc, nil
	}
	//wait for all the SSO pods to be ready
	for _, pod := range podList.Items {
		for _, condition := range pod.Status.Conditions {
			if condition.Type == "Ready" && condition.Status != "True" {
				return kc, nil
			}
		}
	}
	//get the route to keycloak admin
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
	kc.Status.Phase = v1alpha1.PhaseReconcile
	return kc, nil
}

func (ph *phaseHandler) Reconcile(sso *v1alpha1.Keycloak) (*v1alpha1.Keycloak, error) {
	multiError := &util.MultiError{}
	sso, err := ph.reconcileDBPassword(sso)
	if err != nil {
		multiError.AddError(errors.Wrap(err, "could not reconcile db password"))
	}

	sso, err = ph.reconcileMonitoringResources(sso)
	if err != nil {
		multiError.AddError(errors.Wrap(err, "could not reconcile monitoring resources"))
	}

	sso, err = ph.reconcileBackups(sso)
	if err != nil {
		multiError.AddError(errors.Wrap(err, "could not reconcile backups"))
	}
	if multiError.IsNil() {
		return sso, nil
	}

	return sso, multiError

}

func (ph *phaseHandler) reconcileBackups(sso *v1alpha1.Keycloak) (*v1alpha1.Keycloak, error) {
	multiError := &util.MultiError{}

	for _, backup := range sso.Spec.Backups {
		err := ph.reconcileBackup(sso, backup, sso.Namespace)
		if err != nil {
			multiError.AddError(err)
		}
	}
	if !multiError.IsNil() {
		return sso, multiError
	}
	return sso, nil
}

func (ph *phaseHandler) reconcileBackup(sso *v1alpha1.Keycloak, backup v1alpha1.KeycloakBackup, namespace string) error {
	cron := &v1beta1.CronJob{
		ObjectMeta: v12.ObjectMeta{
			Name:   backup.Name,
			Labels: map[string]string{"application": "sso", "sso": sso.Name},
		},
		Spec: v1beta1.CronJobSpec{
			Schedule: backup.Schedule,
			JobTemplate: v1beta1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: v1.PodTemplateSpec{
						Spec: v1.PodSpec{
							Containers: []v1.Container{
								{
									Name:    backup.Name + "-keycloak-backup",
									Image:   backup.Image + ":" + backup.ImageTag,
									Command: []string{"/opt/intly/tools/entrypoint.sh", "-c", "postgres", "-b", "s3", "-e", "gpg"},
									EnvFrom: []v1.EnvFromSource{
										{
											SecretRef: &v1.SecretEnvSource{
												LocalObjectReference: v1.LocalObjectReference{
													Name: backup.AwsCredentialsSecretName,
												},
											},
										},
										{
											SecretRef: &v1.SecretEnvSource{
												LocalObjectReference: v1.LocalObjectReference{
													Name: "db-credentials-" + sso.Name,
												},
											},
										},
										{
											SecretRef: &v1.SecretEnvSource{
												LocalObjectReference: v1.LocalObjectReference{
													Name: backup.EncryptionKeySecretName,
												},
											},
										},
									},
								},
							},
							RestartPolicy: v1.RestartPolicyOnFailure,
						},
					},
				},
			},
		},
	}
	_, err := ph.k8sClient.BatchV1beta1().CronJobs(namespace).Create(cron)
	if err != nil && !errors2.IsAlreadyExists(err) {
		return errors.Wrapf(err, "error creating cronjob %s/%s", cron.Namespace, cron.Name)
	}
	if err != nil && errors2.IsAlreadyExists(err) {
		_, err := ph.k8sClient.BatchV1beta1().CronJobs(namespace).Update(cron)
		if err != nil {
			return errors.Wrapf(err, "could not update cronjob %s/%s", cron.Namespace, cron.Name)
		}
	}

	return nil
}

func (ph *phaseHandler) reconcileDBPassword(sso *v1alpha1.Keycloak) (*v1alpha1.Keycloak, error) {
	ssoDc, err := ph.ocDCClient.DeploymentConfigs(sso.Namespace).Get("sso", v12.GetOptions{})
	if err != nil {
		return sso, errors.Wrap(err, "could not get 'sso' deploymentconfig")
	}

	username := ""
	password := ""
	host := "sso-postgresql." + sso.Namespace + ".svc"

	for _, envVar := range ssoDc.Spec.Template.Spec.Containers[0].Env {
		if envVar.Name == "DB_USERNAME" {
			username = envVar.Value
		}
		if envVar.Name == "DB_PASSWORD" {
			password = envVar.Value
		}
	}
	data := map[string][]byte{"POSTGRES_USERNAME": []byte(username), "POSTGRES_PASSWORD": []byte(password), "POSTGRES_HOST": []byte(host)}
	dbCredentialsSecret := &v1.Secret{
		TypeMeta: v12.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: v12.ObjectMeta{
			Labels:    map[string]string{"application": "sso", "sso": sso.Name},
			Namespace: sso.Namespace,
			Name:      "db-credentials-" + sso.Name,
		},
		Data: data,
		Type: "Opaque",
	}
	_, err = ph.k8sClient.CoreV1().Secrets(sso.Namespace).Create(dbCredentialsSecret)
	if err != nil && !errors2.IsAlreadyExists(err) {
		return sso, errors.Wrap(err, "could not create db credentials secret")
	}
	if err != nil && errors2.IsAlreadyExists(err) {
		logrus.Infof("updating secret: %s", dbCredentialsSecret.Name)
		_, err = ph.k8sClient.CoreV1().Secrets(sso.Namespace).Update(dbCredentialsSecret)
		if err != nil {
			return sso, errors.Wrap(err, "could not update db credentials secret")
		}
	}

	return sso, nil

}

func (ph *phaseHandler) reconcileMonitoringResources(sso *v1alpha1.Keycloak) (*v1alpha1.Keycloak, error) {
	kc := sso.DeepCopy()
	if kc.Status.MonitoringResourcesCreated == false {
		created, err := ph.reconcileMonitoringResource(kc, GrafanaDashboardName)
		if err != nil || !created {
			return kc, err
		}

		created, err = ph.reconcileMonitoringResource(kc, ServiceMonitorName)
		if err != nil || !created {
			return kc, err
		}

		created, err = ph.reconcileMonitoringResource(kc, PrometheusRuleName)
		if err != nil || !created {
			return kc, err
		}

		kc.Status.MonitoringResourcesCreated = true
		return kc, nil
	}

	return kc, nil
}

func (ph *phaseHandler) reconcileMonitoringResource(sso *v1alpha1.Keycloak, resource string) (bool, error) {
	created, err := ph.createResource(sso, resource)
	if err != nil {
		return false, err
	}

	if created {
		logrus.Info(fmt.Sprintf("Monitoring resource '%s' successfully created", resource))
		return true, nil
	}

	logrus.Warn(fmt.Sprintf("Cannot create monitoring resource '%s' at this time, retrying", resource))
	return false, nil
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
	// delete cronjobs
	if err := ph.k8sClient.BatchV1beta1().CronJobs(kc.Namespace).DeleteCollection(deleteOpts, listOpts); err != nil {
		return nil, errors.Wrap(err, "failed to delete all cronjobs for sso")
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

// Creates a generic kubernetes resource from a templates
func (ph *phaseHandler) createResource(sso *v1alpha1.Keycloak, resourceName string) (bool, error) {
	kc := sso.DeepCopy()
	resourceHelper := newResourceHelper(kc)
	resource, err := resourceHelper.createResource(resourceName)
	if err != nil {
		return false, err
	}

	gvk := resource.GetObjectKind().GroupVersionKind()
	apiVersion, kind := gvk.ToAPIVersionAndKind()
	resourceClient, _, err := ph.dynamicResourceClientFactory(apiVersion, kind, kc.Namespace)
	if err != nil {
		// The resource cannot be created because the CRD is not installed in the cluster.
		// We can try again later.
		return false, nil
	}

	resource, err = resourceClient.Create(resource)
	if err != nil && !errors2.IsAlreadyExists(err) {
		return false, errors.Wrap(err, "failed to create unstructured object")
	}

	return true, nil
}
