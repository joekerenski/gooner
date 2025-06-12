package config

import (
    "fmt"
    "strings"
    "github.com/spf13/viper"
)

type Config struct {
    Server struct {
        Port    string `mapstructure:"port"`
        Host    string `mapstructure:"host"`
        Name    string `mapstructure:"name"`
    } `mapstructure:"server"`

    Database struct {
        Type     string `mapstructure:"type"`     // "sqlite3" or "postgres"
        Database string `mapstructure:"database"` // db name or file path
        Host     string `mapstructure:"host"`
        Port     int    `mapstructure:"port"`
        User     string `mapstructure:"user"`
        Password string `mapstructure:"password"`
        SSLMode  string `mapstructure:"ssl_mode"`
    } `mapstructure:"database"`

    Auth struct {
        JWTSecret     string `mapstructure:"jwt_secret"`
        RefreshSecret string `mapstructure:"refresh_secret"`
        Pepper        string `mapstructure:"pepper"`
        TokenExpiry   string `mapstructure:"token_expiry"`
        RefreshExpiry string `mapstructure:"refresh_expiry"`
    } `mapstructure:"auth"`
    
    OAuth struct {
        GoogleClientID     string `mapstructure:"google_client_id"`
        GoogleClientSecret string `mapstructure:"google_client_secret"`
        RedirectURL        string `mapstructure:"redirect_url"`
    } `mapstructure:"oauth"`
    
    Stripe struct {
        PublicKey    string `mapstructure:"public_key"`
        SecretKey    string `mapstructure:"secret_key"`
        WebhookSecret string `mapstructure:"webhook_secret"`
    } `mapstructure:"stripe"`
    
    Webhooks struct {
        Secret   string `mapstructure:"secret"`
        Timeout  string `mapstructure:"timeout"`
    } `mapstructure:"webhooks"`
}

func Load() (*Config, error) {
    viper.SetConfigName("config")
    viper.SetConfigType("yaml")
    viper.AddConfigPath(".")
    viper.AddConfigPath("./config")
    
    // Environment variable support
    viper.AutomaticEnv()
    viper.SetEnvPrefix("APP")
    viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
    
    // Set defaults
    setDefaults()
    
    // Read config file (optional - fallback to env vars)
    if err := viper.ReadInConfig(); err != nil {
        if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
            return nil, fmt.Errorf("error reading config file: %w", err)
        }
    }
    
    var config Config
    if err := viper.Unmarshal(&config); err != nil {
        return nil, fmt.Errorf("unable to decode config: %w", err)
    }
    
    return &config, nil
}

func setDefaults() {
    viper.SetDefault("server.port", "8000")
    viper.SetDefault("server.host", "localhost")
    viper.SetDefault("server.name", "GOONER")
    viper.SetDefault("database.type", "sqlite3")
    viper.SetDefault("database.database", "app.db")
    viper.SetDefault("database.ssl_mode", "disable")
    viper.SetDefault("auth.token_expiry", "24h")
    viper.SetDefault("auth.refresh_expiry", "168h")
    viper.SetDefault("webhooks.timeout", "30s")
}
