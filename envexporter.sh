#!/usr/bin/env bash
set -a  # automatically export all variables

# Ensure yq is installed (only once)
if ! command -v yq >/dev/null 2>&1; then
  echo "Installing yq..."
  brew install yq
fi

CONFIG_FILE="./internal/config/config.yaml"

# ENV
export ENV=$(yq -r '.ENV' $CONFIG_FILE)

# DB
export POSTGRES_USER=$(yq -r '.DB.PG_USER' $CONFIG_FILE)
export POSTGRES_PASSWORD=$(yq -r '.DB.PG_PASS' $CONFIG_FILE)
export POSTGRES_DB=$(yq -r '.DB.PG_DB_NAME' $CONFIG_FILE)
export POSTGRES_PORT=$(yq -r '.DB.PG_PORT' $CONFIG_FILE)
export POSTGRES_HOST=$(yq -r '.DB.PG_HOST' $CONFIG_FILE)

# REDIS
export REDIS_PORT=$(yq -r '.REDIS.REDIS_PORT' $CONFIG_FILE)
export REDIS_HOST=$(yq -r '.REDIS.REDIS_HOST' $CONFIG_FILE)
export REDIS_QUEUE_NAME=$(yq -r '.REDIS.REDIS_QUEUE_NAME' $CONFIG_FILE)

# S3 (MinIO)
export MINIO_ROOT_USER=$(yq -r '.S3.MINIO_ACCESS_KEY' $CONFIG_FILE)
export MINIO_ROOT_PASSWORD=$(yq -r '.S3.MINIO_SECRET_KEY' $CONFIG_FILE)
export MINIO_ENDPOINT=$(yq -r '.S3.MINIO_ENDPOINT' $CONFIG_FILE)
export MINIO_BUCKET=$(yq -r '.S3.MINIO_BUCKET' $CONFIG_FILE)

# SERVER
export HTTP_PORT=$(yq -r '.SERVER.HTTP_PORT' $CONFIG_FILE)

set +a
