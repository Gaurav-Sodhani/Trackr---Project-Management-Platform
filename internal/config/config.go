package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	ServerPort string
}

func Load() *Config {
	// .env is optional -- env vars take precedence (for Docker)
	godotenv.Load()

	port := getEnv("SERVER_PORT", "8080")
	// Render sets PORT env var for hosted services
	if p := os.Getenv("PORT"); p != "" {
		port = p
	}

	return &Config{
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "pmuser"),
		DBPassword: getEnv("DB_PASSWORD", "pmpass"),
		DBName:     getEnv("DB_NAME", "project_mgmt"),
		ServerPort: port,
	}
}

func (c *Config) DSN() string {
	// Render provides DATABASE_URL directly
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		return dbURL
	}
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		c.DBHost, c.DBPort, c.DBUser, c.DBPassword, c.DBName,
	)
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
