# Keycloak Realms

The keycloak realm CRD is used to define a realm within a keycloak instance, this includes all users, clients and identity providers for that realm.

## Example Realm

To see an example keycloak realm definition, take a look at [this example](./deploy/examples/keycloakRealm.json).

## Explanation of Values

Some values in this CRD are used to inform the operator how to behave, and most are used as values to the keycloak instance API.

The values used to inform the operator and listed and documented below. 

For more information on the values used by the Keycloak API, please see their [API documentation](https://www.keycloak.org/docs-api/2.5/rest-api/).

### Index of Operator Values

- users/outputSecret
- users/password
- clients/outputSecret

#### Users / OutputSecret

This value is used by the Operator to output the credentials used to create a user in the keycloak instance. It will be created in the same namespace as the keycloakRealm CR.

#### Users / Password

This value is optional, if this is present it used as the password for this user during creation, and then removed from the CRD. If this is not present the user is assigned a random password. In either case, the resulting credentials are stored in the users outputSecret.

#### Clients / OutputSecret

This value is used by the operator to store the client secret and installation string, it is created and maintained by the operator in the same namespace as the keycloakRealm CR.
