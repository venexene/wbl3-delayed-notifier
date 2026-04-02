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
    RabbitURL string
}

func Load() (*Config, error) {
    err := godotenv.Load()
	if err != nil {
		log.Println(".env not found, using system env")
	}
    
    port := os.Getenv("HTTP_PORT")
    if port == "" {
        port = "8080"
    }
    
    db_user := os.Getenv("DB_USER")
    if db_user == "" {
        return nil, fmt.Errorf("DB_USER is required")
    }

    db_pass := os.Getenv("DB_PASSWORD")
    if db_pass == "" {
        return nil, fmt.Errorf("DB_PASSWORD is required")
    }

    db_port := os.Getenv("DB_PORT")
    if db_port == "" {
        return nil, fmt.Errorf("DB_PORT is required")
    }

    db_name := os.Getenv("DB_NAME")
    if db_name == "" {
        return nil, fmt.Errorf("DB_USER is required")
    }

    rb_port := os.Getenv("RABBIT_PORT")
    if rb_port == "" {
        return nil, fmt.Errorf("RABBIT_PORT is required")
    }

    rb_user := os.Getenv("RABBIT_USER")
    if rb_user == "" {
        return nil, fmt.Errorf("RABBIT_USER is required")
    }

    rb_pass := os.Getenv("RABBIT_PASSWORD")
    if rb_pass == "" {
        return nil, fmt.Errorf("RABBIT_PASSWORD is required")
    }

    dsn := fmt.Sprintf("postgres://%s:%s@localhost:%s/%s",
        db_user,
        db_pass,
        db_port,
        db_name,
    )


    rurl := fmt.Sprintf("amqp://%s:%s@localhost:%s/",
        rb_user,
        rb_pass,
        rb_port,
    )

    
    return &Config{
        HTTPPort: port,
        DB_DSN: dsn,
        RabbitURL: rurl,
    }, nil
}