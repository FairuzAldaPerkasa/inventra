package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	AppName  string
	HTTPAddr string
}

func Load() (Config, error) {
	cfg := Config{
		AppName:  getEnv("APP_NAME", "Inventra"),
		HTTPAddr: getEnv("HTTP_ADDR", "127.0.0.1:8081"),
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

	return cfg, nil
}

func getEnv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	return value
}
