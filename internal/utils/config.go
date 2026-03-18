package utils

import "os"

type EnvConfig struct {
	SERVER_PORT           string
	SERVER_URL            string
	CLIENT_URL            string
	DATABASE_URL          string
	AWS_BUCKET_REGION     string
	AWS_BUCKET_NAME       string
	AMQP_URL              string
	AWS_ACCESS_KEY_ID     string
	AWS_SECRET_ACCESS_KEY string
	GOOGLE_CLIENT_ID      string
	GOOGLE_CLIENT_SECRET  string
	JWT_KEY               string
	AES_BLOCK_KEY         string
}

func NewEnvConfig() *EnvConfig {
	return &EnvConfig{
		SERVER_PORT:           os.Getenv("SERVER_PORT"),
		SERVER_URL:            os.Getenv("SERVER_URL"),
		CLIENT_URL:            os.Getenv("CLIENT_URL"),
		DATABASE_URL:          os.Getenv("DATABASE_URL"),
		AWS_BUCKET_REGION:     os.Getenv("AWS_BUCKET_REGION"),
		AWS_BUCKET_NAME:       os.Getenv("AWS_BUCKET_NAME"),
		AMQP_URL:              os.Getenv("AMQP_URL"),
		AWS_ACCESS_KEY_ID:     os.Getenv("AWS_ACCESS_KEY_ID"),
		AWS_SECRET_ACCESS_KEY: os.Getenv("AWS_SECRET_ACCESS_KEY"),
		GOOGLE_CLIENT_ID:      os.Getenv("GOOGLE_CLIENT_ID"),
		GOOGLE_CLIENT_SECRET:  os.Getenv("GOOGLE_CLIENT_SECRET"),
		JWT_KEY:               os.Getenv("JWT_KEY"),
		AES_BLOCK_KEY:         os.Getenv("AES_BLOCK_KEY"),
	}
}
