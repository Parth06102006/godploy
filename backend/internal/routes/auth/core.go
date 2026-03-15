package authroutes

import (
	"context"

	"github.com/Roshan-anand/godploy/internal/config"
	"github.com/go-playground/validator/v10"
)

type AuthHandler struct {
	Server   *config.Server
	Validate *validator.Validate
	qCtx     context.Context
}

func InitAuthHandlers(s *config.Server) *AuthHandler {
	return &AuthHandler{
		Server:   s,
		Validate: validator.New(),
		qCtx:      context.Background(),
	}
}
