package config

import (
	"flag"
	"fmt"
	"os"
	"sync"

	"gopkg.in/yaml.v3"
)

var (
	cfg  *Config
	once sync.Once
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
	var err error
	once.Do(func() {
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
	return cfg, err
}

func GetConfig() (*Config, error) {
	if cfg == nil {
		return load()
	}
	return cfg, nil
}
