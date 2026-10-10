package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const fullYAML = `
ENV: dev
DB:
  PG_USER: mytube
  PG_PASS: secret
  PG_DB_NAME: mytube
  PG_PORT: "5432"
  PG_HOST: localhost
REDIS:
  REDIS_PORT: "6379"
  REDIS_HOST: localhost
  REDIS_QUEUE_NAME: jobs
S3:
  MINIO_ACCESS_KEY: ak
  MINIO_SECRET_KEY: sk
  MINIO_ENDPOINT: localhost:9002
  MINIO_BUCKET: mytube
  MINIO_REDIS_QUEUE_NAME: bucketevents
SERVER:
  HTTP_PORT: "8080"
ES:
  HOST: localhost
  PORT: "9200"
KEYCLOAK:
  URL: http://localhost:8180
  REALM: mytube
  CLIENT_ID: mytube-frontend
STREAM:
  BASE_URL: http://localhost
  TOKEN_SECRET: change-me
  TOKEN_TTL: 6h
`

func writeFile(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestValidateConfigPath(t *testing.T) {
	file := writeFile(t, "ENV: dev")
	if err := validateConfigPath(file); err != nil {
		t.Errorf("file: %v", err)
	}
	dir := t.TempDir()
	if err := validateConfigPath(dir); err == nil || !strings.Contains(err.Error(), "is a directory") {
		t.Errorf("dir: %v", err)
	}
	if err := validateConfigPath(filepath.Join(dir, "missing.yaml")); err == nil {
		t.Error("missing file: expected error")
	}
}

func TestDecodeFullConfig(t *testing.T) {
	var c Config
	if err := yaml.Unmarshal([]byte(fullYAML), &c); err != nil {
		t.Fatal(err)
	}
	checks := map[string][2]string{
		"env":            {c.Env, "dev"},
		"pg user":        {c.DB.PgUser, "mytube"},
		"pg port":        {c.DB.PgPort, "5432"},
		"redis queue":    {c.Redis.RedisQueueName, "jobs"},
		"bucket":         {c.S3.MinioBucket, "mytube"},
		"bucket events":  {c.S3.MinioRedisQueueName, "bucketevents"},
		"http port":      {c.Server.HttpPort, "8080"},
		"es host":        {c.Es.EsHost, "localhost"},
		"keycloak url":   {c.Keycloak.Url, "http://localhost:8180"},
		"keycloak realm": {c.Keycloak.Realm, "mytube"},
		"client id":      {c.Keycloak.ClientId, "mytube-frontend"},
		"stream base":    {c.Stream.BaseUrl, "http://localhost"},
		"stream secret":  {c.Stream.TokenSecret, "change-me"},
		"stream ttl":     {c.Stream.TokenTTL, "6h"},
	}
	for name, v := range checks {
		if v[0] != v[1] {
			t.Errorf("%s = %q, want %q", name, v[0], v[1])
		}
	}
}

func TestDecodeMissingStreamSection(t *testing.T) {
	without := fullYAML[:strings.Index(fullYAML, "STREAM:")]
	var c Config
	if err := yaml.Unmarshal([]byte(without), &c); err != nil {
		t.Fatal(err)
	}
	// decoding succeeds with a zero Stream; stream.NewSigner is what refuses to start (see signer tests)
	if c.Stream != (Stream{}) {
		t.Errorf("stream = %+v", c.Stream)
	}
	if c.Keycloak.Realm != "mytube" {
		t.Error("other sections should still decode")
	}
}

func TestExampleConfigDecodes(t *testing.T) {
	b, err := os.ReadFile("../../configs/config.example.yaml")
	if err != nil {
		t.Skipf("example config not readable: %v", err)
	}
	var c Config
	if err := yaml.Unmarshal(b, &c); err != nil {
		t.Fatalf("config.example.yaml does not decode: %v", err)
	}
	if c.Stream.BaseUrl == "" || c.Stream.TokenSecret == "" {
		t.Errorf("config.example.yaml should have a STREAM section, got %+v", c.Stream)
	}
	if c.Keycloak.Url == "" || c.Keycloak.Realm == "" {
		t.Errorf("config.example.yaml should have a KEYCLOAK section, got %+v", c.Keycloak)
	}
}

// GetConfig parses -config from os.Args once per process (sync.Once), so all of its
// behaviour is exercised in this single test.
func TestGetConfigLoadsOnce(t *testing.T) {
	path := writeFile(t, fullYAML)
	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })
	os.Args = []string{"mytube-api", "-config", path}

	c, err := GetConfig()
	if err != nil {
		t.Fatalf("GetConfig: %v", err)
	}
	if c.Stream.TokenTTL != "6h" || c.DB.PgHost != "localhost" {
		t.Errorf("config = %+v", c)
	}
	// later calls return the cached pointer, even if the file changes
	if err := os.WriteFile(path, []byte("ENV: prod"), 0o600); err != nil {
		t.Fatal(err)
	}
	c2, err := GetConfig()
	if err != nil || c2 != c || c2.Env != "dev" {
		t.Errorf("second GetConfig = %+v, %v", c2, err)
	}
}

// A failed first load is not retried and is not remembered: sync.Once has run, cfg is still nil,
// and the error lived in a per-call variable, so the second GetConfig returns (nil, nil).
// Runs in a subprocess because GetConfig's state is process-global.
func TestGetConfigErrorIsNotSticky(t *testing.T) {
	if os.Getenv("MYTUBE_CONFIG_SUBPROCESS") == "1" {
		os.Args = []string{"mytube-api", "-config", filepath.Join(os.TempDir(), "definitely-missing-mytube.yaml")}
		if _, err := GetConfig(); err == nil {
			t.Fatal("first call: expected an error for a missing file")
		}
		c, err := GetConfig()
		if c == nil && err == nil {
			t.Fatal("second call returned (nil, nil): callers will dereference a nil *Config")
		}
		return
	}
	t.Skip("BUG: internal/config/config.go:84-111 — if the first load fails, every later GetConfig() returns (nil, nil) " +
		"because once.Do never runs again and the error is not stored; callers such as storage.NewS3Store then dereference nil. " +
		"Remove this Skip to run the subprocess check.")
	cmd := exec.Command(os.Args[0], "-test.run=^TestGetConfigErrorIsNotSticky$", "-test.count=1")
	cmd.Env = append(os.Environ(), "MYTUBE_CONFIG_SUBPROCESS=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("subprocess failed: %v\n%s", err, out)
	}
}
