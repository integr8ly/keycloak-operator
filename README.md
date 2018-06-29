# Keycloak Operator

**Status** proof of concept


This is a kubernets operator based on the Operator pattern and uses the operator sdk.

# Custom Resource Types Supported

- Keycloak: represents a keycloak server for this operator to interact with
 - may support a cluster flag that will indicate to the operator that this should be added to a cluster of keycloaks
- KeycloakRealm: represents a keycloak realm. When it is created the operator will
set up a realm and user in the referenced Keycloak. It will then store the credentials
for this realm in a new secret named after the realm
- KeycloakClient: represents a keycloak client. When created the operator will
setup a client of the given type in the specified realm and store the result in a secret
named after the realm-client 
- KeycloakClientSync: when created the operator will sync the existing clients within a realm to secrets in the namespace
