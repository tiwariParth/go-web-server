package config

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DBHost     string
	DBUser     string
	DBPassword string
	DBName     string
	DBPort     string
	ServerPort string
}

func LoadConfig() *Config {
	// Load .env file if it exists
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found: %v", err)
	}

	config := &Config{
		DBHost:     getEnvWithDefault("DB_HOST", "localhost"),
		DBUser:     getEnvWithDefault("DB_USER", "postgres"),
		DBPassword: getEnvWithDefault("DB_PASSWORD", ""),
		DBName:     getEnvWithDefault("DB_NAME", "crud_demo"),
		DBPort:     getEnvWithDefault("DB_PORT", "5432"),
		ServerPort: getEnvWithDefault("SERVER_PORT", "8080"),
	}

	return config
}

func (c *Config) GetDSN() string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		c.DBHost, c.DBPort, c.DBUser, c.DBPassword, c.DBName)
}

func getEnvWithDefault(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}