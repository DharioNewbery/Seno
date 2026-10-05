// Package httpapi concentra as rotas Gin, handlers e middlewares
// (autenticação, RBAC) da API, conforme ARQUITETURA §3.
package httpapi

import "github.com/gin-gonic/gin"

// RegisterRoutes monta o roteador da API com o envelope de erros, as rotas
// da fundação e as rotas de autenticação (ARQUITETURA §5).
func RegisterRoutes(r *gin.Engine, deps *Dependencies) {
	r.Use(RequestLog(), OriginGuard(deps.Cfg.WebOrigin))

	r.GET("/healthz", deps.Healthz)

	v1 := r.Group("/v1")
	v1.GET("/healthz", deps.Healthz)

	auth := v1.Group("/auth")
	auth.POST("/login", deps.Login)
	auth.POST("/password/reset-request", deps.RequestPasswordReset)
	auth.POST("/password/reset", deps.ResetPassword)

	autenticado := auth.Group("")
	autenticado.Use(deps.RequireAuth())
	autenticado.POST("/logout", deps.Logout)
	autenticado.GET("/me", deps.Me)
}
