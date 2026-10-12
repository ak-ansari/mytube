#!/usr/bin/env bash
# Deploys the whole MyTube stack to the local minikube and exposes it on a free public HTTPS URL
# (Cloudflare quick tunnel). Idempotent: re-run it after code changes or whenever the tunnel URL changed
# (laptop sleep, minikube restart, cloudflared pod restart).
#   SKIP_BUILD=1 ./up.sh     re-wire only, no image rebuild
set -euo pipefail
cd "$(dirname "$0")"
ROOT=../..
FRONTEND=${FRONTEND_DIR:-../../web}
NS=mytube
SECRETS=.secrets.env
k() { kubectl -n "$NS" "$@"; }
step() { printf '\n\033[1;34m==> %s\033[0m\n' "$*"; }

step "minikube"
minikube status >/dev/null 2>&1 || minikube start --cpus=4 --memory=7000
kubectl config use-context minikube >/dev/null

if [ ! -f "$SECRETS" ]; then
  step "generating $SECRETS (keep it; it holds the passwords)"
  rnd() { openssl rand -hex 16; }
  cat > "$SECRETS" <<EOF
PG_PASSWORD=$(rnd)
MINIO_USER=mytube
MINIO_PASSWORD=$(rnd)
VIDEO_TOKEN_SECRET=$(rnd)
KEYCLOAK_ADMIN_PASSWORD=$(rnd)
EOF
fi
set -a; . "./$SECRETS"; set +a

# Third-party images go through your Mac's Docker: pulls from inside minikube can stall, and
# minio/minio is no longer published on Docker Hub (your local copy is used).
step "preloading images into minikube"
IN_CLUSTER=$(minikube image ls)
for img in minio/minio:latest postgres:15 redis:7 docker.elastic.co/elasticsearch/elasticsearch:9.2.1 nginx:1.27 cloudflare/cloudflared:2025.9.1; do
  grep -q "${img%%:*}:${img##*:}" <<<"$IN_CLUSTER" && continue
  docker image inspect "$img" >/dev/null 2>&1 || docker pull "$img"
  echo "loading $img"; minikube image load "$img"
done

if [ -z "${SKIP_BUILD:-}" ]; then
  step "building images inside minikube"
  eval "$(minikube docker-env)"
  docker build -t mytube-backend:local -f Dockerfile "$ROOT"
  docker build -t mytube-keycloak:local -f keycloak.Dockerfile "$ROOT/deployments/keycloak"
  docker build -t mytube-frontend:local -f frontend.Dockerfile "$FRONTEND"
  eval "$(minikube docker-env --unset)"
fi

step "infra + tunnel"
kubectl apply -f manifests/00-namespace.yaml
k create secret generic mytube-secrets --from-env-file="$SECRETS" --dry-run=client -o yaml | kubectl apply -f -
k create configmap edge-conf --from-file=edge.conf.template --dry-run=client -o yaml | kubectl apply -f -
for f in 10-postgres 11-redis 12-minio 14-elasticsearch 15-keycloak 22-frontend 30-edge 31-cloudflared; do
  kubectl apply -f "manifests/$f.yaml"
done
k delete job minio-setup --ignore-not-found >/dev/null
kubectl apply -f manifests/13-minio-setup.yaml
# images are rebuilt under the same tag, so restart to pick them up
[ -z "${SKIP_BUILD:-}" ] && k rollout restart deploy/frontend deploy/keycloak >/dev/null
k rollout restart deploy/edge >/dev/null   # re-reads edge.conf.template

step "waiting for the tunnel URL"
HOST=""
for _ in $(seq 1 60); do
  HOST=$(k logs deploy/cloudflared 2>/dev/null | grep -o 'https://[a-z0-9-]*\.trycloudflare\.com' | tail -1 | sed 's#https://##' || true)
  [ -n "$HOST" ] && break
  sleep 3
done
[ -n "$HOST" ] || { echo "no tunnel URL in cloudflared logs"; k logs deploy/cloudflared | tail -20; exit 1; }
echo "public host: $HOST"

step "rendering API/worker config"
CFG=$(mktemp)
trap 'rm -f "$CFG"' EXIT
cat > "$CFG" <<EOF
ENV: prod
DB:
  PG_USER: mytube
  PG_PASS: "$PG_PASSWORD"
  PG_DB_NAME: mytube
  PG_PORT: "5432"
  PG_HOST: postgres
REDIS:
  REDIS_PORT: "6379"
  REDIS_HOST: redis
  REDIS_QUEUE_NAME: video_queue
S3:
  MINIO_ACCESS_KEY: "$MINIO_USER"
  MINIO_SECRET_KEY: "$MINIO_PASSWORD"
  MINIO_ENDPOINT: minio:9000
  MINIO_BUCKET: mytube
  MINIO_REDIS_QUEUE_NAME: minio-events-queue
  MINIO_PUBLIC_ENDPOINT: "$HOST"
  MINIO_PUBLIC_SECURE: true
SERVER:
  HTTP_PORT: "8080"
ES:
  PORT: "9200"
  HOST: elasticsearch
KEYCLOAK:
  URL: https://$HOST/kc
  REALM: mytube
  CLIENT_ID: mytube-frontend
STREAM:
  BASE_URL: https://$HOST
  TOKEN_SECRET: "$VIDEO_TOKEN_SECRET"
  TOKEN_TTL: 6h
EOF
k create secret generic mytube-config --from-file=config.yaml="$CFG" --dry-run=client -o yaml | kubectl apply -f -

step "waiting for infra"
for d in postgres redis minio elasticsearch keycloak edge frontend; do k rollout status deploy/$d --timeout=10m; done
k wait --for=condition=complete job/minio-setup --timeout=5m

step "pointing the Keycloak client at https://$HOST"
k exec deploy/keycloak -- env HOST="$HOST" bash -c '
  set -e
  kc=/opt/keycloak/bin/kcadm.sh
  $kc config credentials --server http://localhost:8080/kc --realm master --user admin --password "$KC_BOOTSTRAP_ADMIN_PASSWORD" >/dev/null
  id=$($kc get clients -r mytube -q clientId=mytube-frontend --fields id --format csv --noquotes)
  $kc update clients/$id -r mytube \
    -s "rootUrl=https://$HOST" \
    -s "redirectUris=[\"https://$HOST/*\"]" \
    -s "webOrigins=[\"+\"]" \
    -s "attributes.\"post.logout.redirect.uris\"=https://$HOST/*"
  $kc update realms/mytube -s loginTheme=mytube'

step "waiting until https://$HOST answers (DNS can take a minute)"
until curl -sf -o /dev/null "https://$HOST/kc/realms/mytube"; do sleep 5; done

step "API + worker"
kubectl apply -f manifests/20-api.yaml
k rollout restart deploy/api >/dev/null
k rollout status deploy/api --timeout=5m
kubectl apply -f manifests/21-worker.yaml
k rollout restart deploy/worker >/dev/null
k rollout status deploy/worker --timeout=5m

cat <<EOF

✅ MyTube is live:      https://$HOST
   Keycloak admin:      https://$HOST/kc/admin   (admin / $KEYCLOAK_ADMIN_PASSWORD)
   Demo login:          demo / demo123
   Logs:                kubectl -n $NS logs -f deploy/api   (or worker, keycloak, edge, cloudflared)
The URL changes when the tunnel restarts; just run ./up.sh again (SKIP_BUILD=1 to skip the image build).
EOF
