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
	MinioAccessKey string `yaml:"MINIO_ACCESS_KEY"`
	MinioSecretKey string `yaml:"MINIO_SECRET_KEY"`
	MinioEndpoint  string `yaml:"MINIO_ENDPOINT"`
	MinioBucket    string `yaml:"MINIO_BUCKET"`
}
type Server struct {
	HttpPort string `yaml:"HTTP_PORT"`
}
type Config struct {
	DB     DB     `yaml:"DB"`
	Redis  Redis  `yaml:"REDIS"`
	S3     S3     `yaml:"S3"`
	Server Server `yaml:"SERVER"`
	Env    string `yaml:"ENV"`
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
	flag.StringVar(&configPath, "config", "./config.yaml", "path to config file")
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
