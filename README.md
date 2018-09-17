# Keycloak Operator

A Kubernetes Operator based on the Operator SDK for syncing resources in Keycloak.

# Supported Custom Resources 

### Keycloak 
Represents a keycloak server for the Operator to interact with.
The Operator reconciles resources in Keycloak to match the spec defined in the custom resource (an example of this can be found in `/deploy/examples/keycloak.json`). 

The following Keycloak resources are supported:
- `Realms`: Realms are created, updated and deleted in Keycloak to match the custom resource spec
- `Users`:  Users are created and updated in Keycloak to match the custom resource spec
- `Clients`: Clients are created, updated and deleted in Keycloak to match the custom resource spec
- `Identity Providers`: Identity providers are created, updated and deleted in Keycloak to match the custom resource spec

# Test it locally

*Note*: You will need a running Kubernetes or OpenShift cluster to use the Operator

- clone this repo to `$GOPATH/src/github.com/aerogear/keycloak-operator`
- run `make setup install run`

Note that you only need to run `setup` the first time. After that you can simply run `make run`.

You should see something like:

```go
INFO[0000] Go Version: go1.10.2
INFO[0000] Go OS/Arch: darwin/amd64
INFO[0000] operator-sdk Version: 0.0.5+git

```

In a new terminal run `make create-examples`. In the original terminal you should see output like:

```go
handling object  aerogear.org/v1alpha1, Kind=Keycloak
handling object  aerogear.org/v1alpha1, Kind=SharedServiceAction
handling object  aerogear.org/v1alpha1, Kind=SharedServiceSlice
handling object  aerogear.org/v1alpha1, Kind=SharedService
```

# Deploying to a Cluster

Before the Keycloak Operator can be deployed to a running cluster, the necessary Custom Resource Definitions (CRDs) must be installed. To do this, run `make install_crds`.

The Keycloak Operator needs to be deployed into the same namespace as the [managed-services-broker](https://github.com/aerogear/managed-services-broker).

To deploy to this namespace, you must first create the RBAC in that namespace:
```
oc create -f deploy/rbac.yaml -n <managed-services-broker-namespace>
```

Now that the namespace is ready to run the Keycloak Operator you can create the deployment for it:
```
oc create -f deploy/operator.yaml -n <managed-services-broker-namespace>
```

Finally you will need to create a `SharedService` object for the Keycloak Operator to act upon. There is a sample in this repo that can be used:
```
oc create -f deploy/examples/sharedservice.json -n <managed-services-broker-namespace>
```

Or alternatively create your own.

## Deploying using operator lifecycle manager
[operator lifecycle manager](https://github.com/operator-framework/operator-lifecycle-manager) manages operators and other resources.
To deploy this operator on OLM enabled cluster apply manifest file, edit line 5 of the manifest file to desired namespace:

`kubectl apply -f deploy/olm-catalog/csv.yaml` 

Create operator CRD and RBAC rules:
- `kubectl apply -f deploy/Keycloak_crd.yaml`
- `kubectl apply -f deploy/rbac.yaml`
- `kubectl apply -f deploy/operator.yaml`
# Tear it down

```make uninstall```
