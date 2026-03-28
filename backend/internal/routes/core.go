package routes

import (
	"github.com/Roshan-anand/godploy/frontend"
	"github.com/Roshan-anand/godploy/internal/config"
	"github.com/Roshan-anand/godploy/internal/handlers"
	"github.com/Roshan-anand/godploy/internal/middleware"
	"github.com/labstack/echo/v5"
)

// setup all routes
func SetupRoutes(srv *config.Server) (*echo.Echo, error) {
	h := handlers.NewHandeler(srv)
	m := middleware.NewMiddlewares(srv)
	e := echo.New()

	// initialize static file serving route
	uiFs, err := frontend.GetEmbedFS()
	if err != nil {
		return nil, err
	}
	e.StaticFS("/", uiFs)

	e.Use(m.GlobalMiddlewareCors())

	// health check route
	e.GET("/api/health", h.Health.HealthCheck)

	// initialize auth api routes
	authApi := e.Group("/api/auth")
	authApi.GET("/user", h.Auth.AuthUser, m.GlobalMiddlewareUser)
	authApi.POST("/register", h.Auth.AppRegiter)
	authApi.POST("/login", h.Auth.AppLogin)

	// secured routes
	api := e.Group("/api")
	api.Use(m.GlobalMiddlewareUser)

	projectApi := api.Group("/project")
	projectApi.GET("", h.Project.GetProjects)
	projectApi.POST("", h.Project.CreateProject)
	projectApi.DELETE("", h.Project.DeleteProject)

	serviceApi := api.Group("/service")
	serviceApi.POST("/psql", h.Service.CreatePsqlService)
	serviceApi.DELETE("/psql", h.Service.DeletePsqlService)
	serviceApi.POST("/psql/deploy", h.Service.DeployPsqlService)
	serviceApi.POST("/psql/stop", h.Service.StopPsqlService)

	ghApi := api.Group("/provider/github")
	ghApi.GET("/app/create", h.Git.CreateGithubApp)
	ghApi.GET("/app/callback", h.Git.CreateGithubAppCallback)
	ghApi.GET("/app/setup", h.Git.SetupGithubApp)
	ghApi.GET("/repo/list", h.Git.GetGithubRepoList)

	return e, nil
}
