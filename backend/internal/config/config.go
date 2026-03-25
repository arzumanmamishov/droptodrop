package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all application configuration.
type Config struct {
	Server   ServerConfig
	Shopify  ShopifyConfig
	Database DatabaseConfig
	Redis    RedisConfig
	Session  SessionConfig
	Worker   WorkerConfig
	Security SecurityConfig
}

type ServerConfig struct {
	Host string
	Port string
	Env  string
}

type ShopifyConfig struct {
	APIKey      string
	APISecret   string
	Scopes      string
	AppURL      string
	RedirectURI string
}

type DatabaseConfig struct {
	URL             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

type RedisConfig struct {
	URL        string
	Password   string
	MaxRetries int
}

type SessionConfig struct {
	Secret string
	MaxAge int
}

type WorkerConfig struct {
	Concurrency int
	RetryMax    int
	RetryDelay  time.Duration
}

type SecurityConfig struct {
	EncryptionKey  string
	RateLimitRPS   int
	RateLimitBurst int
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Host: getEnv("SERVER_HOST", "0.0.0.0"),
			Port: getEnv("PORT", getEnv("SERVER_PORT", "8080")),
			Env:  getEnv("APP_ENV", "development"),
		},
		Shopify: ShopifyConfig{
			APIKey:      requireEnv("SHOPIFY_API_KEY"),
			APISecret:   requireEnv("SHOPIFY_API_SECRET"),
			Scopes:      getEnv("SHOPIFY_SCOPES", "read_products,write_products,read_orders,write_orders,read_fulfillments,write_fulfillments,read_inventory,write_inventory,read_shipping"),
			AppURL:      requireEnv("SHOPIFY_APP_URL"),
			RedirectURI: requireEnv("SHOPIFY_REDIRECT_URI"),
		},
		Database: DatabaseConfig{
			URL:             requireEnv("DATABASE_URL"),
			MaxOpenConns:    getEnvInt("DATABASE_MAX_OPEN_CONNS", 25),
			MaxIdleConns:    getEnvInt("DATABASE_MAX_IDLE_CONNS", 5),
			ConnMaxLifetime: getEnvDuration("DATABASE_CONN_MAX_LIFETIME", 5*time.Minute),
		},
		Redis: RedisConfig{
			URL:        getEnv("REDIS_URL", "redis://localhost:6379/0"),
			Password:   getEnv("REDIS_PASSWORD", ""),
			MaxRetries: getEnvInt("REDIS_MAX_RETRIES", 3),
		},
		Session: SessionConfig{
			Secret: requireEnv("SESSION_SECRET"),
			MaxAge: getEnvInt("SESSION_MAX_AGE", 86400),
		},
		Worker: WorkerConfig{
			Concurrency: getEnvInt("WORKER_CONCURRENCY", 5),
			RetryMax:    getEnvInt("WORKER_RETRY_MAX", 3),
			RetryDelay:  getEnvDuration("WORKER_RETRY_DELAY", 5*time.Second),
		},
		Security: SecurityConfig{
			EncryptionKey:  requireEnv("ENCRYPTION_KEY"),
			RateLimitRPS:   getEnvInt("RATE_LIMIT_RPS", 100),
			RateLimitBurst: getEnvInt("RATE_LIMIT_BURST", 200),
		},
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("config validation: %w", err)
	}

	return cfg, nil
}

func (c *Config) validate() error {
	if c.Shopify.APIKey == "" {
		return fmt.Errorf("SHOPIFY_API_KEY is required")
	}
	if c.Shopify.APISecret == "" {
		return fmt.Errorf("SHOPIFY_API_SECRET is required")
	}
	if len(c.Security.EncryptionKey) < 32 {
		return fmt.Errorf("ENCRYPTION_KEY must be at least 32 characters")
	}
	return nil
}

func (c *Config) IsDevelopment() bool {
	return c.Server.Env == "development"
}

func (c *Config) IsProduction() bool {
	return c.Server.Env == "production"
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func requireEnv(key string) string {
	val := os.Getenv(key)
	if val == "" {
		// Return empty; validate() will catch required fields
		return ""
	}
	return val
}

func getEnvInt(key string, fallback int) int {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	i, err := strconv.Atoi(val)
	if err != nil {
		return fallback
	}
	return i
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	d, err := time.ParseDuration(val)
	if err != nil {
		return fallback
	}
	return d
}
