package handlers

import (
	"context"

	"github.com/Roshan-anand/godploy/internal/config"
	"github.com/Roshan-anand/godploy/internal/lib"
	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v5"
)

type HealthHandler struct {
	Server   *config.Server
	Validate *validator.Validate
	qCtx     context.Context
}

func InitHealthHandlers(s *config.Server) *HealthHandler {
	return &HealthHandler{
		Server:   s,
		Validate: validator.New(),
		qCtx:     context.Background(),
	}
}

// to check server health and connectivity with database and other dependencies
//
// route: GET /api/health
func (h *HealthHandler) HealthCheck(c *echo.Context) error {
	if h.Server.DB == nil {
		return c.JSON(500, lib.Res{Message: "database not initialized"})
	}
	return c.JSON(200, lib.Res{Message: "ok"})
}
