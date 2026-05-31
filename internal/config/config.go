package config

import (
	"os"
	"time"

	"github.com/caarlos0/env/v11"
)

type Config struct {
	ServerPort     int           `env:"SERVER_PORT" envDefault:"8080"`
	DatabaseURL    string        `env:"DATABASE_URL,required"`
	JWTSecret      string        `env:"JWT_SECRET,required"`
	JWTAccessTTL   time.Duration `env:"JWT_ACCESS_TTL" envDefault:"15m"`
	JWTRefreshTTL  time.Duration `env:"JWT_REFRESH_TTL" envDefault:"720h"`
	S3Bucket       string        `env:"S3_BUCKET" envDefault:"messenger-files"`
	S3Region       string        `env:"S3_REGION" envDefault:"us-east-1"`
	S3Endpoint     string        `env:"S3_ENDPOINT"`
	FileMaxSize    int64         `env:"FILE_MAX_SIZE" envDefault:"52428800"`
	InviteTTL      time.Duration `env:"INVITE_TTL" envDefault:"168h"`
	VAPIDPublicKey  string       `env:"VAPID_PUBLIC_KEY"`
	VAPIDPrivateKey string       `env:"VAPID_PRIVATE_KEY"`
	VAPIDContact    string       `env:"VAPID_CONTACT"`

	// WSAllowedOrigins is the allowlist of Origin headers accepted for the
	// WebSocket endpoint. When empty, any origin is accepted (dev convenience).
	WSAllowedOrigins []string `env:"WS_ALLOWED_ORIGINS" envSeparator:","`

	// Redis powers the cross-replica WebSocket event bridge. When RedisAddr is
	// empty the bridge is disabled and realtime delivery stays in-process.
	RedisAddr     string `env:"REDIS_ADDR"`
	RedisPassword string `env:"REDIS_PASSWORD"`
	RedisDB       int    `env:"REDIS_DB" envDefault:"0"`

	// InstanceID identifies this replica in broker messages so it can ignore
	// its own published events. Defaults to the hostname when unset.
	InstanceID string `env:"INSTANCE_ID"`
}

func Load() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, err
	}
	if cfg.InstanceID == "" {
		cfg.InstanceID, _ = os.Hostname()
	}
	return cfg, nil
}
