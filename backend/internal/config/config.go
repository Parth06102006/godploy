package config

import (
	"fmt"
	"os"
)

type Config struct {
	Port             string
	SessionDataName  string
	SessionTokenName string
	EchoCtxUserKey   string
	JwtSecret        string
	AllowedCors      []string
	DbDir            string
	AppEnv           string
}

func LoadConfig() *Config {
	appEnv := os.Getenv("APP_ENV")
	jwtSecrect := os.Getenv("JWT_SECRET")
	fmt.Println("env : ", appEnv)
	fmt.Println("env : ", jwtSecrect)

	// TODO: load from env variable
	return &Config{
		Port:             "8080",
		SessionDataName:  "godploy_session_data",
		SessionTokenName: "godploy_session_token",
		EchoCtxUserKey:   "user_email",
		JwtSecret:        jwtSecrect,
		AllowedCors:      []string{"http://localhost:5173"},
		DbDir:            "data",
		AppEnv:           appEnv,
	}
}
