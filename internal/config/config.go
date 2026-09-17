package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppName          string
	HTTPAddr         string
	DBPassword       string
	RabbitMQUser     string
	RabbitMQPassword string
	RabbitMQVHost    string
	SessionTTL       time.Duration
	CookieSecure     bool
	CookieDomain     string
}

func Load() (Config, error) {
	cfg := Config{
		AppName:          getEnv("APP_NAME", "Inventra"),
		HTTPAddr:         getEnv("HTTP_ADDR", "127.0.0.1:8081"),
		DBPassword:       os.Getenv("DB_PASSWORD"),
		RabbitMQUser:     getEnv("RABBITMQ_USER", "inventra_app"),
		RabbitMQPassword: os.Getenv("RABBITMQ_PASSWORD"),
		RabbitMQVHost:    getEnv("RABBITMQ_VHOST", "inventra"),
		CookieDomain:     getEnv("COOKIE_DOMAIN", ""),
	}
	if cfg.DBPassword == "" {
		return Config{}, fmt.Errorf("DB_PASSWORD wajib diisi")
	}
	if cfg.RabbitMQPassword == "" {
		return Config{}, fmt.Errorf("RABBITMQ_PASSWORD wajib diisi")
	}

	host, port, err := net.SplitHostPort(cfg.HTTPAddr)
	if err != nil {
		return Config{}, fmt.Errorf(
			"HTTP_ADDR harus berformat host:port: %w", err,
		)
	}
	if host == "" {
		return Config{}, fmt.Errorf("host pada HTTP_ADDR tidak boleh kosong")
	}

	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return Config{}, fmt.Errorf(
			"port pada HTTP_ADDR harus berupa angka 1–65535",
		)
	}

	sessionTTLHours, err := strconv.Atoi(getEnv("SESSION_TTL_HOURS", "24"))
	if err != nil || sessionTTLHours < 1 {
		return Config{}, fmt.Errorf(
			"SESSION_TTL_HOURS harus berupa angka positif",
		)
	}
	cfg.SessionTTL = time.Duration(sessionTTLHours) * time.Hour

	cookieSecure, err := strconv.ParseBool(getEnv("COOKIE_SECURE", "true"))
	if err != nil {
		return Config{}, fmt.Errorf("COOKIE_SECURE harus true/false")
	}
	cfg.CookieSecure = cookieSecure

	return cfg, nil
}

func getEnv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	return value
}
