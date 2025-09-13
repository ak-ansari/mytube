SHELL := /bin/bash
CONFIG=internal/config/config.yaml
ENV_SCRIPT=./envexporter.sh

.PHONY: all docker-up setup-bucket run-api run-worker down

# Default target (runs when you just type `make`)
all: docker-up setup-bucket

docker-up:
	@echo "Starting docker-compose services..."
	@bash -c "source $(ENV_SCRIPT) && docker-compose up -d"

setup-bucket:
	@echo "Configuring MinIO bucket..."
	@bash -c "source $(ENV_SCRIPT) && \
	mc alias set myminio http://localhost:9002 \$${MINIO_ROOT_USER} \$${MINIO_ROOT_PASSWORD} && \
	mc mb --ignore-existing myminio/\$${MINIO_BUCKET} && \
	mc anonymous set download myminio/\$${MINIO_BUCKET}"


run-api:
	@bash -c "source $(ENV_SCRIPT) && ENV=$$ENV go run ./cmd/api/main.go -config $(CONFIG)"

run-worker:
	@bash -c "source $(ENV_SCRIPT) && ENV=$$ENV go run ./cmd/worker/main.go -config $(CONFIG)"

down:
	@echo "Stopping docker services..."
	@docker-compose down
	@echo "✅ All infra shut down. Stop API/Worker manually with Ctrl-C in their tabs."
