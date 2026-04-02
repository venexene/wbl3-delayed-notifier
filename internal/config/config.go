package config

import (
    "os"
    "fmt"
    "log"
    
    "github.com/joho/godotenv"
)

type Config struct {
    HTTPPort string
    DB_DSN   string
}

func Load() (*Config, error) {
    err := godotenv.Load()
	if err != nil {
		log.Println(".env not found, using system env")
	}
    
    port := os.Getenv("HTTPPort")
    if port == "" {
        port = "8080"
    }
    
    dsn := os.Getenv("DB_DSN")
    if dsn == "" {
        return nil, fmt.Errorf("DB_DSN is required")
    }

    return &Config{
        HTTPPort: port,
        DB_DSN: dsn,
    }, nil
}