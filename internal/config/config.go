package config

import (
	"errors"
	"io/fs"
	"log"
	"net"
	"net/url"
	"os"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
	"github.com/joho/godotenv"
)

type Config struct {
	Env         string `yaml:"env" env-default:"local"`
	AliasLength int    `yaml:"alias_length" env-default:"6"`
	HTTPServer  `yaml:"http_server"`
	Postgres    Postgres
	Auth        Auth
}

type HTTPServer struct {
	Address     string        `yaml:"address" env-default:"0.0.0.0:8082"`
	Timeout     time.Duration `yaml:"timeout" env-default:"4s"`
	IdleTimeout time.Duration `yaml:"idle_timeout" env-default:"60s"`
}

type Postgres struct {
	Host     string `env:"POSTGRES_HOST" env-required:"true"`
	Port     string `env:"POSTGRES_PORT" env-default:"5432"`
	User     string `env:"POSTGRES_USER" env-required:"true"`
	Password string `env:"POSTGRES_PASSWORD" env-required:"true"`
	DB       string `env:"POSTGRES_DB" env-required:"true"`
	SSLMode  string `env:"POSTGRES_SSLMODE" env-default:"disable"`
}

type Auth struct {
	User     string `env:"AUTH_USER" env-required:"true"`
	Password string `env:"AUTH_PASSWORD" env-required:"true"`
}

func (p Postgres) DSN() string {
	q := url.Values{}
	q.Set("sslmode", p.SSLMode)

	q.Set("connect_timeout", "2")

	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(p.User, p.Password),
		Host:     net.JoinHostPort(p.Host, p.Port),
		Path:     p.DB,
		RawQuery: q.Encode(),
	}

	return u.String()
}

func MustLoad() *Config {

	if err := godotenv.Load(); err != nil && !errors.Is(err, fs.ErrNotExist) {
		log.Fatalf("cannot load .env: %s", err)
	}

	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		log.Fatal("CONFIG_PATH is not set")
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		log.Fatalf("config file does not exist: %s", configPath)
	}

	var cfg Config

	if err := cleanenv.ReadConfig(configPath, &cfg); err != nil {
		log.Fatalf("cannot read config: %s", err)
	}

	if cfg.AliasLength <= 0 {
		log.Fatalf("alias_length must be positive, got %d", cfg.AliasLength)
	}

	return &cfg
}
