package main

import (
	"context"
	"runtime"
	"strings"
	"time"

	"github.com/operator-framework/operator-sdk/pkg/sdk"
	sdkVersion "github.com/operator-framework/operator-sdk/version"

	"flag"

	"os"

	"github.com/aerogear/keycloak-operator/pkg/apis/aerogear/v1alpha1"
	"github.com/aerogear/keycloak-operator/pkg/dispatch"
	"github.com/aerogear/keycloak-operator/pkg/k8s"
	"github.com/aerogear/keycloak-operator/pkg/keycloak"
	"github.com/aerogear/keycloak-operator/pkg/keycloak/realm"
	"github.com/operator-framework/operator-sdk/pkg/k8sclient"
	"github.com/operator-framework/operator-sdk/pkg/util/k8sutil"
	"github.com/sirupsen/logrus"
	// Load Openshift types
	_ "github.com/aerogear/keycloak-operator/pkg/apis/openshift"
)

func printVersion() {
	logrus.Infof("Go Version: %s", runtime.Version())
	logrus.Infof("Go OS/Arch: %s/%s", runtime.GOOS, runtime.GOARCH)
	logrus.Infof("operator-sdk Version: %v", sdkVersion.Version)
	logrus.Infof("operator config: resync: %v, sync-resources: %v", cfg.ResyncPeriod, cfg.SyncResources)
}

var (
	cfg v1alpha1.Config
)

func init() {
	flagset := flag.CommandLine
	flagset.IntVar(&cfg.ResyncPeriod, "resync", 60, "change the resync period")
	flagset.StringVar(&cfg.LogLevel, "log-level", logrus.Level.String(logrus.InfoLevel), "Log level to use. Possible values: panic, fatal, error, warn, info, debug")
	flagset.BoolVar(&cfg.SyncResources, "sync-resources", true, "Sync Keycloak resources on each reconciliation loop after the initial creation of the realm.")
	flagset.Parse(os.Args[1:])
}

func main() {

	logLevel, err := logrus.ParseLevel(cfg.LogLevel)
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
	k8Client := k8sclient.GetKubeClient()
	kcFactory := &keycloak.KeycloakFactory{SecretClient: k8Client.CoreV1().Secrets(namespace)}

	resyncDuration := time.Second * time.Duration(cfg.ResyncPeriod)
	sdk.Watch(resource, v1alpha1.KeycloakKind, namespace, resyncDuration)
	for _, ns := range strings.Split(os.Getenv("CONSUMER_NAMESPACES"), ";") {
		sdk.Watch(resource, v1alpha1.KeycloakRealmKind, ns, resyncDuration)
	}

	dh := dispatch.NewHandler(k8Client)
	dispatcher := dh.(*dispatch.Handler)

	// setup cruder
	cruder := k8s.Cruder{}
	// Handle keycloak resource reconcile
	dispatcher.AddHandler(keycloak.NewReconciler(kcFactory, k8Client, cruder))
	dispatcher.AddHandler(realm.NewRealmHandler(kcFactory, cruder, realm.NewPhaseHandler(k8Client, cruder, namespace, kcFactory)))

	// main dispatch of resources
	sdk.Handle(dispatcher)
	sdk.Run(context.TODO())
}
