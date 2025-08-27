# MyTube

A minimal, runnable pipeline for a YouTube-like backend in Go using:
- **MinIO** for object storage (S3 compatible)
- **Postgres** for metadata
- **Redis Streams** for the job queue
- **FFmpeg/FFprobe** for media processing

## Requirements
- Docker + Docker Compose (for infra)
- FFmpeg/FFprobe installed on your machine (used by the worker)

## Run infra
```bash
docker-compose up -d
