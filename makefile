SHELL := /bin/bash
CONFIG=configs/config.yaml
ENV_SCRIPT=./scripts/envexporter.sh
COMPOSE=docker-compose -f deployments/docker-compose.yml

.PHONY: all docker-up setup-bucket keycloak-dev run-api run-worker down

# Default target (runs when you just type `make`)
all: docker-up setup-bucket

docker-up:
	@echo "Starting docker-compose services..."
	@bash -c "source $(ENV_SCRIPT) && $(COMPOSE) up -d"

setup-bucket:
	@echo "Configuring MinIO bucket..."
	@bash -c "source $(ENV_SCRIPT) && \
	mc alias set myminio http://localhost:9002 \$${MINIO_ROOT_USER} \$${MINIO_ROOT_PASSWORD} && \
	mc mb --ignore-existing myminio/\$${MINIO_BUCKET} && \
	mc anonymous set download myminio/\$${MINIO_BUCKET}"


# Keycloak's master realm requires HTTPS for non-localhost callers; through Docker's NAT the host counts as
# external, which blocks the admin console at http://localhost:8180. Run once after Keycloak starts (admin/admin).
keycloak-dev:
	@docker exec keycloak bash -c '/opt/keycloak/bin/kcadm.sh config credentials --server http://localhost:8080 --realm master --user admin --password admin && /opt/keycloak/bin/kcadm.sh update realms/master -s sslRequired=NONE'
	@echo "✅ Keycloak admin console: http://localhost:8180 (admin/admin)"

run-api:
	@bash -c "source $(ENV_SCRIPT) && ENV=$$ENV go run ./cmd/api/main.go -config $(CONFIG)"

run-worker:
	@bash -c "source $(ENV_SCRIPT) && ENV=$$ENV go run ./cmd/worker/main.go -config $(CONFIG)"

down:
	@echo "Stopping docker services..."
	@bash -c "source $(ENV_SCRIPT) && $(COMPOSE) down"
	@echo "✅ All infra shut down. Stop API/Worker manually with Ctrl-C in their tabs."
