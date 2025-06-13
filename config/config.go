package config

import (
    "fmt"
    "os"
    "gopkg.in/yaml.v3"
)

type Config struct {
    Server struct {
        Port string `yaml:"port" env:"APP_SERVER_PORT"`
        Host string `yaml:"host" env:"APP_SERVER_HOST"`
        Name string `yaml:"name" env:"APP_SERVER_NAME"`
    } `yaml:"server"`

    Database struct {
        Type     string `yaml:"type" env:"APP_DATABASE_TYPE"`
        Database string `yaml:"database" env:"APP_DATABASE_DATABASE"`
        Host     string `yaml:"host" env:"APP_DATABASE_HOST"`
        Port     int    `yaml:"port" env:"APP_DATABASE_PORT"`
        User     string `yaml:"user" env:"APP_DATABASE_USER"`
        Password string `yaml:"password" env:"APP_DATABASE_PASSWORD"`
        SSLMode  string `yaml:"ssl_mode" env:"APP_DATABASE_SSL_MODE"`
    } `yaml:"database"`

    Auth struct {
        JWTSecret     string `yaml:"jwt_secret" env:"APP_AUTH_JWT_SECRET"`
        RefreshSecret string `yaml:"refresh_secret" env:"APP_AUTH_REFRESH_SECRET"`
        Pepper        string `yaml:"pepper" env:"APP_AUTH_PEPPER"`
        TokenExpiry   string `yaml:"token_expiry" env:"APP_AUTH_TOKEN_EXPIRY"`
        RefreshExpiry string `yaml:"refresh_expiry" env:"APP_AUTH_REFRESH_EXPIRY"`
    } `yaml:"auth"`

    OAuth struct {
        GoogleClientID     string `yaml:"google_client_id" env:"APP_OAUTH_GOOGLE_CLIENT_ID"`
        GoogleClientSecret string `yaml:"google_client_secret" env:"APP_OAUTH_GOOGLE_CLIENT_SECRET"`
        RedirectURL        string `yaml:"redirect_url" env:"APP_OAUTH_REDIRECT_URL"`
    } `yaml:"oauth"`

    Stripe struct {
        PublicKey     string `yaml:"public_key" env:"APP_STRIPE_PUBLIC_KEY"`
        SecretKey     string `yaml:"secret_key" env:"APP_STRIPE_SECRET_KEY"`
        WebhookSecret string `yaml:"webhook_secret" env:"APP_STRIPE_WEBHOOK_SECRET"`
    } `yaml:"stripe"`

    Webhooks struct {
        Secret  string `yaml:"secret" env:"APP_WEBHOOKS_SECRET"`
        Timeout string `yaml:"timeout" env:"APP_WEBHOOKS_TIMEOUT"`
    } `yaml:"webhooks"`
}

func Load() (*Config, error) {
    config := &Config{}

    setDefaults(config)

    if data, err := os.ReadFile("config.yaml"); err == nil {
        if err := yaml.Unmarshal(data, config); err != nil {
            return nil, fmt.Errorf("error parsing config.yaml: %w", err)
        }
    } else {
        if data, err := os.ReadFile("./config/config.yaml"); err == nil {
            if err := yaml.Unmarshal(data, config); err != nil {
                return nil, fmt.Errorf("error parsing config/config.yaml: %w", err)
            }
        }
    }

    // overrideWithEnv(config)

    return config, nil
}

func setDefaults(config *Config) {
    config.Server.Port = "8000"
    config.Server.Host = "localhost"
    config.Server.Name = "GOONER"
    config.Database.Type = "sqlite3"
    config.Database.Database = "app.db"
    config.Database.SSLMode = "disable"
    config.Auth.TokenExpiry = "24h"
    config.Auth.RefreshExpiry = "168h"
    config.Webhooks.Timeout = "30s"
}

func overrideWithEnv(config *Config) {
    if val := os.Getenv("APP_SERVER_PORT"); val != "" {
        config.Server.Port = val
    }
    if val := os.Getenv("APP_SERVER_HOST"); val != "" {
        config.Server.Host = val
    }
    if val := os.Getenv("APP_SERVER_NAME"); val != "" {
        config.Server.Name = val
    }
}
