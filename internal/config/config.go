package config

import "os"

type Config struct {
	Port       string
	DBHost     string
	DBPort     string
	DBName     string
	DBUser     string
	DBPassword string
}

func Load() Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dbHost := os.Getenv("DB_HOST")
	if dbHost == "" {
		dbHost = "localhost"
	}

	dbPort := os.Getenv("DB_PORT")
	if dbPort == "" {
		dbPort = "5432"
	}

	dbName := os.Getenv("DB_NAME")
	if dbName == "" {
		dbName = "stocks"
	}

	dbUser := os.Getenv("DB_USER")
	if dbUser == "" {
		dbUser = "stocks"
	}

	dbPassword := os.Getenv("DB_PASSWORD")
	if dbPassword == "" {
		dbPassword = "stocks"
	}

	return Config{
		Port:   port,
		DBHost: dbHost,
		DBPort:     dbPort,
		DBName:     dbName,
		DBUser:     dbUser,
		DBPassword: dbPassword,
	}
}
