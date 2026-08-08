package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Env             string
	HTTPAddr        string
	ShutdownTimeout time.Duration
	Database        Database
	Auth            Auth
	Telemetry       Telemetry
	RateLimit       RateLimit
	Events          Events
	Relay           Relay
	Temporal        Temporal
}

type Temporal struct {
	HostPort  string
	Namespace string
	TaskQueue string
}

type Events struct {
	NATSURL       string
	StreamName    string
	SubjectPrefix string
}

type Relay struct {
	BatchSize    int
	PollInterval time.Duration
}

type Database struct {
	URL             string
	MaxConns        int32
	MinConns        int32
	MaxConnLifetime time.Duration
	MaxConnIdleTime time.Duration
}

type Auth struct {
	Mode       string
	HMACSecret string
	JWKSURL    string
	Issuer     string
	Audience   string
}

type Telemetry struct {
	ServiceName  string
	OTLPEndpoint string
}

type RateLimit struct {
	RPS   float64
	Burst int
}

func (c Config) IsProduction() bool { return c.Env == "production" }

func Load() (Config, error) {
	cfg := Config{
		Env:             getStr("FORGE_ENV", "development"),
		HTTPAddr:        getStr("FORGE_HTTP_ADDR", ":8080"),
		ShutdownTimeout: getDur("FORGE_SHUTDOWN_TIMEOUT", 15*time.Second),
		Database: Database{
			URL:             getStr("FORGE_DB_URL", ""),
			MaxConns:        int32(getInt("FORGE_DB_MAX_CONNS", 10)),
			MinConns:        int32(getInt("FORGE_DB_MIN_CONNS", 2)),
			MaxConnLifetime: getDur("FORGE_DB_MAX_CONN_LIFETIME", time.Hour),
			MaxConnIdleTime: getDur("FORGE_DB_MAX_CONN_IDLE_TIME", 30*time.Minute),
		},
		Auth: Auth{
			Mode:       getStr("FORGE_AUTH_MODE", "hmac"),
			HMACSecret: getStr("FORGE_AUTH_HMAC_SECRET", ""),
			JWKSURL:    getStr("FORGE_AUTH_JWKS_URL", ""),
			Issuer:     getStr("FORGE_AUTH_ISSUER", ""),
			Audience:   getStr("FORGE_AUTH_AUDIENCE", ""),
		},
		Telemetry: Telemetry{
			ServiceName:  getStr("FORGE_OTEL_SERVICE_NAME", "forge-control-plane"),
			OTLPEndpoint: getStr("FORGE_OTEL_OTLP_ENDPOINT", ""),
		},
		RateLimit: RateLimit{
			RPS:   getFloat("FORGE_RATE_LIMIT_RPS", 50),
			Burst: getInt("FORGE_RATE_LIMIT_BURST", 100),
		},
		Events: Events{
			NATSURL:       getStr("FORGE_NATS_URL", "nats://localhost:4222"),
			StreamName:    getStr("FORGE_EVENTS_STREAM", "FORGE_EVENTS"),
			SubjectPrefix: getStr("FORGE_EVENTS_SUBJECT_PREFIX", "forge.events"),
		},
		Relay: Relay{
			BatchSize:    getInt("FORGE_RELAY_BATCH_SIZE", 100),
			PollInterval: getDur("FORGE_RELAY_POLL_INTERVAL", time.Second),
		},
		Temporal: Temporal{
			HostPort:  getStr("FORGE_TEMPORAL_HOSTPORT", "localhost:7233"),
			Namespace: getStr("FORGE_TEMPORAL_NAMESPACE", "default"),
			TaskQueue: getStr("FORGE_TEMPORAL_TASK_QUEUE", "forge-provisioning"),
		},
	}
	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) validate() error {
	var problems []string
	if c.Database.URL == "" {
		problems = append(problems, "FORGE_DB_URL is required")
	}
	switch c.Auth.Mode {
	case "hmac":
		if c.Auth.HMACSecret == "" {
			problems = append(problems, "FORGE_AUTH_HMAC_SECRET is required when FORGE_AUTH_MODE=hmac")
		}
		if c.IsProduction() {
			problems = append(problems, "FORGE_AUTH_MODE=hmac is not permitted in production; use jwks")
		}
	case "jwks":
		if c.Auth.JWKSURL == "" {
			problems = append(problems, "FORGE_AUTH_JWKS_URL is required when FORGE_AUTH_MODE=jwks")
		}
		if c.Auth.Issuer == "" || c.Auth.Audience == "" {
			problems = append(problems, "FORGE_AUTH_ISSUER and FORGE_AUTH_AUDIENCE are required when FORGE_AUTH_MODE=jwks")
		}
	default:
		problems = append(problems, fmt.Sprintf("FORGE_AUTH_MODE %q is invalid (want hmac or jwks)", c.Auth.Mode))
	}
	if c.Database.MinConns > c.Database.MaxConns {
		problems = append(problems, "FORGE_DB_MIN_CONNS must not exceed FORGE_DB_MAX_CONNS")
	}
	if len(problems) > 0 {
		return fmt.Errorf("invalid configuration: %s", strings.Join(problems, "; "))
	}
	return nil
}

func getStr(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func getInt(key string, def int) int {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func getFloat(key string, def float64) float64 {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if n, err := strconv.ParseFloat(v, 64); err == nil {
			return n
		}
	}
	return def
}

func getDur(key string, def time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}
