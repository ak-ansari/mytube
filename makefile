SHELL := /bin/bash
CONFIG=configs/config.yaml
COMPOSE=docker-compose -f deployments/docker-compose.yml
# app stack + Grafana/Prometheus/Loki/Alloy (deployments/docker-compose.observability.yml)
OBS_COMPOSE=$(COMPOSE) -f deployments/docker-compose.observability.yml
# must match deployments/docker-compose.yml (minio service) and configs/config.yaml (S3.*)
MINIO_USER=mytube
MINIO_PASSWORD=mytube123
MINIO_BUCKET=mytube

.PHONY: all docker-up obs-up obs-down setup-bucket keycloak-dev run-api run-worker down

# Default target (runs when you just type `make`)
all: docker-up setup-bucket

docker-up:
	@echo "Starting docker-compose services..."
	@$(COMPOSE) up -d

obs-up:
	@mkdir -p logs
	@$(OBS_COMPOSE) up -d
	@echo "✅ Grafana http://localhost:3000 (admin/admin) · Prometheus http://localhost:9090 · Alloy http://localhost:12345"

obs-down:
	@$(OBS_COMPOSE) stop prometheus loki alloy grafana postgres-exporter redis-exporter

# The bucket is private. segments/* is anonymously readable only on the docker network (MinIO's :9000 is not
# published); the public path to it is the nginx gateway on :80, which checks a signed token per video.
setup-bucket:
	@echo "Configuring MinIO bucket..."
	@mc alias set myminio http://localhost:9002 $(MINIO_USER) $(MINIO_PASSWORD)
	@mc mb --ignore-existing myminio/$(MINIO_BUCKET)
	@mc anonymous set none myminio/$(MINIO_BUCKET)
	@mc anonymous set download myminio/$(MINIO_BUCKET)/segments/

# Keycloak's master realm requires HTTPS for non-localhost callers; through Docker's NAT the host counts as
# external, which blocks the admin console at http://localhost:8180. Run once after Keycloak starts (admin/admin).
keycloak-dev:
	@docker exec keycloak bash -c '/opt/keycloak/bin/kcadm.sh config credentials --server http://localhost:8080 --realm master --user admin --password admin && /opt/keycloak/bin/kcadm.sh update realms/master -s sslRequired=NONE && /opt/keycloak/bin/kcadm.sh update realms/mytube -s loginTheme=mytube'
	@echo "✅ Keycloak admin console: http://localhost:8180 (admin/admin)"

run-api:
	@go run ./cmd/api/main.go -config $(CONFIG)

run-worker:
	@go run ./cmd/worker/main.go -config $(CONFIG)

down:
	@echo "Stopping docker services..."
	@$(OBS_COMPOSE) down
	@echo "✅ All infra shut down. Stop API/Worker manually with Ctrl-C in their tabs."
