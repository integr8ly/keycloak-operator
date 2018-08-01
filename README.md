# Keycloak Operator

This is a kubernets operator based on the Operator pattern and uses the operator sdk.

# Custom Resource Types Supported

- Keycloak: represents a keycloak server for this operator to interact with

# Test It locally

Note you will need a kubernetes or OpenShift cluster

- clone this repo to ```$GOPATH/src/github.com/aerogear/keycloak-operator```
- run ```make setup install run```

Note you only need to run setup the first time. After the first time you can just run ```make run```

You should see something like:

```
INFO[0000] Go Version: go1.10.2
INFO[0000] Go OS/Arch: darwin/amd64
INFO[0000] operator-sdk Version: 0.0.5+git

```

In a new terminal run ```make create-examples```

In the original terminal you should see output like:

```
handling object  aerogear.org/v1alpha1, Kind=Keycloak
handling object  aerogear.org/v1alpha1, Kind=SharedServiceAction
handling object  aerogear.org/v1alpha1, Kind=SharedServiceSlice
handling object  aerogear.org/v1alpha1, Kind=SharedService
```

# Deploying to a Cluster

Before the keycloak-operator can be deployed to a running cluster the necessary Custom Resource Definitions (CRDs) must be installed, to do this, use the following command:
```
make install_crds
```

The keycloak-operator needs to be deployed into the same namespace as the [managed-services-broker](https://github.com/aerogear/managed-services-broker).

To deploy to this namespace, you must first create the RBAC in that namespace:
```
oc create -f deploy/rbac.yaml -n <managed-services-broker-namespace>
```

Now that the namespace is ready to run the keycloak-operator you can create the deployment for it:
```
oc create -f deploy/operator.yaml -n <managed-services-broker-namespace>
```

Finally you will need to create a `SharedService` object for the keycloak-operator to act upon. There is a sample in this repo that can be used:
```
oc create -f deploy/examples/sharedservice.json -n <managed-services-broker-namespace>
```

Or alternatively create your own.

# Tear it down

``` make uninstall```
