package config

import (
	"flag"
	"fmt"
	"os"
	"sync"

	"gopkg.in/yaml.v3"
)

var (
	cfg     *Config
	loadErr error // kept so every GetConfig after a failed first load reports the failure
	once    sync.Once
)

type DB struct {
	PgUser   string `yaml:"PG_USER"`
	PgPASS   string `yaml:"PG_PASS"`
	PgDBName string `yaml:"PG_DB_NAME"`
	PgPort   string `yaml:"PG_PORT"`
	PgHost   string `yaml:"PG_HOST"`
}
type Redis struct {
	RedisPort      string `yaml:"REDIS_PORT"`
	RedisHost      string `yaml:"REDIS_HOST"`
	RedisQueueName string `yaml:"REDIS_QUEUE_NAME"`
}
type S3 struct {
	MinioAccessKey      string `yaml:"MINIO_ACCESS_KEY"`
	MinioSecretKey      string `yaml:"MINIO_SECRET_KEY"`
	MinioEndpoint       string `yaml:"MINIO_ENDPOINT"`
	MinioBucket         string `yaml:"MINIO_BUCKET"`
	MinioRedisQueueName string `yaml:"MINIO_REDIS_QUEUE_NAME"`
	// Optional public origin that presigned URLs point at (e.g. behind a tunnel/ingress); empty = MINIO_ENDPOINT
	MinioPublicEndpoint string `yaml:"MINIO_PUBLIC_ENDPOINT"`
	MinioPublicSecure   bool   `yaml:"MINIO_PUBLIC_SECURE"` // https for MINIO_PUBLIC_ENDPOINT
}
type ES struct {
	EsPort string `yaml:"PORT"`
	EsHost string `yaml:"HOST"`
}
type Keycloak struct {
	Url      string `yaml:"URL"`       // e.g. http://localhost:8180 (must match the "iss" host the frontend logs in through)
	Realm    string `yaml:"REALM"`     // e.g. mytube
	ClientId string `yaml:"CLIENT_ID"` // frontend public client; checked against the token "azp" claim
}

// Stream configures signed HLS playback URLs served by the nginx secure_link gateway.
type Stream struct {
	BaseUrl     string `yaml:"BASE_URL"`     // public origin of the gateway, e.g. http://localhost
	TokenSecret string `yaml:"TOKEN_SECRET"` // shared with nginx (VIDEO_TOKEN_SECRET)
	TokenTTL    string `yaml:"TOKEN_TTL"`    // Go duration, e.g. 6h; must outlive a viewing session
}

// Worker tunes the pipeline worker's retries. Every field is optional (see workers.DefaultRetryPolicy).
type Worker struct {
	MaxAttempts    int    `yaml:"MAX_ATTEMPTS"`     // total runs of a failing stage, including the first
	RetryBaseDelay string `yaml:"RETRY_BASE_DELAY"` // Go duration before the first retry; doubles per retry
	RetryMaxDelay  string `yaml:"RETRY_MAX_DELAY"`  // Go duration cap on a single retry delay
}

// Observability configures the log file Grafana Alloy ships to Loki and the worker's Prometheus endpoint.
// Every field is optional; empty LOG_DIR disables the file sink.
type Observability struct {
	LogDir      string `yaml:"LOG_DIR"`      // JSON logs go to <LOG_DIR>/<api|worker>.log, e.g. ./logs
	MetricsAddr string `yaml:"METRICS_ADDR"` // worker /metrics listen address, e.g. :9091 (the API serves :HTTP_PORT/metrics)
}
type Server struct {
	HttpPort string `yaml:"HTTP_PORT"`
}
type Config struct {
	DB       DB       `yaml:"DB"`
	Redis    Redis    `yaml:"REDIS"`
	S3       S3       `yaml:"S3"`
	Server   Server   `yaml:"SERVER"`
	Env      string   `yaml:"ENV"`
	Es       ES       `yaml:"ES"`
	Keycloak Keycloak `yaml:"KEYCLOAK"`
	Stream   Stream   `yaml:"STREAM"`
	Worker   Worker   `yaml:"WORKER"`

	Observability Observability `yaml:"OBSERVABILITY"`
}

func validateConfigPath(path string) error {
	s, err := os.Stat(path)
	if err != nil {
		return err
	}
	if s.IsDir() {
		return fmt.Errorf("'%s' is a directory not a file", path)
	}
	return nil
}

func parseFlag() (string, error) {
	var configPath string
	flag.StringVar(&configPath, "config", "./configs/config.yaml", "path to config file")
	flag.Parse()
	if err := validateConfigPath(configPath); err != nil {
		return "", err
	}
	return configPath, nil
}

func load() (*Config, error) {
	once.Do(func() {
		var err error
		defer func() { loadErr = err }()
		var configPath string
		configPath, err = parseFlag()
		if err != nil {
			return
		}
		config := &Config{}

		f, e := os.Open(configPath)
		if e != nil {
			err = e
			return
		}
		defer f.Close()

		decoder := yaml.NewDecoder(f)
		if err = decoder.Decode(config); err != nil {
			return
		}
		cfg = config
	})
	return cfg, loadErr
}

func GetConfig() (*Config, error) {
	if cfg == nil {
		return load()
	}
	return cfg, nil
}
