package main

import (
	"context"
	"runtime"

	stub "github.com/aerogear/keycloak-operator/pkg/keycloak"
	"github.com/operator-framework/operator-sdk/pkg/sdk"
	sdkVersion "github.com/operator-framework/operator-sdk/version"

	"github.com/operator-framework/operator-sdk/pkg/k8sclient"
	"github.com/sirupsen/logrus"
)

func printVersion() {
	logrus.Infof("Go Version: %s", runtime.Version())
	logrus.Infof("Go OS/Arch: %s/%s", runtime.GOOS, runtime.GOARCH)
	logrus.Infof("operator-sdk Version: %v", sdkVersion.Version)
}

func main() {
	printVersion()

	resource := "areogear.org/v1alpha1"
	realmkind := "KeycloakRealm"
	clientkind := "KeycloakClient"
	clientsynckind := "KeycloakClientSync"
	keycloakkind := "Keycloak"
	//namespace, err := k8sutil.GetWatchNamespace()
	//if err != nil {
	//	logrus.Fatalf("Failed to get watch namespace: %v", err)
	//}
	namespace := ""
	resyncPeriod := 5
	sdk.Watch(resource, realmkind, namespace, resyncPeriod)
	sdk.Watch(resource, clientkind, namespace, resyncPeriod)
	sdk.Watch(resource, clientsynckind, namespace, resyncPeriod)
	sdk.Watch(resource, keycloakkind, namespace, resyncPeriod)
	k8Client := k8sclient.GetKubeClient()
	sdk.Handle(stub.NewHandler(k8Client))
	sdk.Run(context.TODO())
}
