// Package httpapi concentra as rotas Gin, handlers e middlewares
// (autenticação, RBAC) da API, conforme ARQUITETURA §3.
package httpapi

import "github.com/gin-gonic/gin"

// RegisterRoutes monta o roteador da API com o envelope de erros e as
// rotas da fundação; autenticação e domínio entram nos而言之 passos seguintes.
func RegisterRoutes(r *gin.Engine, deps *Dependencies) {
	r.Use(RequestLog(), OriginGuard(deps.Cfg.WebOrigin))

	r.GET("/healthz", deps.Healthz)

	v1 := r.Group("/v1")
	v1.GET("/healthz", deps.Healthz)
}
