# Keycloak Operator

**Status** early proof of concept not currently doing much


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

# Tear it down

``` make uninstall```
