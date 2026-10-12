# API + worker image (one image, two binaries). Build context: the mytube repo root.
FROM golang:1.25-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/api ./cmd/api && CGO_ENABLED=0 go build -o /out/worker ./cmd/worker

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ffmpeg ca-certificates && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=build /out/api /out/worker ./
# migrations are read from ./migrations relative to the working directory
COPY migrations ./migrations
EXPOSE 8080
CMD ["/app/api", "-config", "/app/configs/config.yaml"]
