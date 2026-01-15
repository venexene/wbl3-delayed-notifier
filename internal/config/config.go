package config

import (
    "os"
    
    "github.com/joho/godotenv"
)

type Config struct {
    HTTPPort string
}

func Load() (*Config, error) {
    _ = godotenv.Load(".env")
    
    port := os.Getenv("HTTPPort")
    if port == "" {
        port = "8080"
    }
        
    return &Config{
        HTTPPort: port,
    }, nil
}