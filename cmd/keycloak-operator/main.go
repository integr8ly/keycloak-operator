package main

import (
	"context"
	"runtime"

	"github.com/operator-framework/operator-sdk/pkg/sdk"
	sdkVersion "github.com/operator-framework/operator-sdk/version"

	"flag"

	"os"

	"github.com/aerogear/keycloak-operator/pkg/apis/aerogear/v1alpha1"
	"github.com/aerogear/keycloak-operator/pkg/dispatch"
	"github.com/aerogear/keycloak-operator/pkg/keycloak"
	"github.com/aerogear/keycloak-operator/pkg/shared"
	sc "github.com/kubernetes-incubator/service-catalog/pkg/client/clientset_generated/clientset"
	"github.com/operator-framework/operator-sdk/pkg/k8sclient"
	"github.com/operator-framework/operator-sdk/pkg/util/k8sutil"
	"github.com/sirupsen/logrus"
)

func printVersion() {
	logrus.Infof("Go Version: %s", runtime.Version())
	logrus.Infof("Go OS/Arch: %s/%s", runtime.GOOS, runtime.GOARCH)
	logrus.Infof("operator-sdk Version: %v", sdkVersion.Version)
}

var (
	resyncFlag *int    = new(int)
	logLevel   *string = new(string)
)

func init() {
	flagset := flag.CommandLine
	flagset.IntVar(resyncFlag, "resync", 7, "change the resync period")
	flagset.StringVar(logLevel, "log-level", logrus.Level.String(logrus.InfoLevel), "Log level to use. Possible values: panic, fatal, error, warn, info, debug")
	flagset.Parse(os.Args[1:])
}

func main() {
	logLevel, err := logrus.ParseLevel(*logLevel)
	if err != nil {
		logrus.Errorf("Failed to parse log level: %v", err)
	} else {
		logrus.SetLevel(logLevel)
	}
	printVersion()
	resource := v1alpha1.Group + "/" + v1alpha1.Version
	namespace, err := k8sutil.GetWatchNamespace()
	if err != nil {
		logrus.Fatalf("Failed to get watch namespace: %v", err)
	}
	cfg := k8sclient.GetKubeConfig()
	svcClient, err := sc.NewForConfig(cfg)
	if err != nil {
		logrus.Fatal("failed to set up service catalog client ", err)
	}
	k8Client := k8sclient.GetKubeClient()
	kcFactory := &keycloak.KeycloakFactory{}

	//set namespace to empty to watch all namespaces
	//namespace := ""
	resyncPeriod := *resyncFlag
	sdk.Watch(resource, v1alpha1.KeycloakKind, namespace, resyncPeriod)
	sdk.Watch(resource, v1alpha1.SharedServiceActionKind, namespace, resyncPeriod)
	sdk.Watch(resource, v1alpha1.SharedServiceKind, namespace, resyncPeriod)
	sdk.Watch(resource, v1alpha1.SharedServiceSliceKind, namespace, resyncPeriod)

	dh := dispatch.NewHandler(k8Client, svcClient)
	dispatcher := dh.(*dispatch.Handler)
	// Handle keycloak resource reconcile
	dispatcher.AddHandler(keycloak.NewHandler(kcFactory, svcClient, k8Client))
	// Handle sharedserviceaction reconcile
	dispatcher.AddHandler(shared.NewServiceActionHandler())
	// Handle sharedservice reconcile
	dispatcher.AddHandler(shared.NewServiceHandler())
	// Handle sharedserviceslice reconcile
	dispatcher.AddHandler(shared.NewServiceSliceHandler())

	// main dispatch of resources
	sdk.Handle(dispatcher)
	sdk.Run(context.TODO())
}
