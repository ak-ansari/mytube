# Keycloak with the mytube realm + login theme baked in. Build context: deployments/keycloak.
FROM quay.io/keycloak/keycloak:26.4
COPY import /opt/keycloak/data/import
COPY themes/mytube /opt/keycloak/themes/mytube
