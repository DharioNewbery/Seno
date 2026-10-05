package httpapi

import (
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/seno-project/seno/api/internal/platform"
)

// RequestLog registra método, caminho, status e duração de cada requisição.
func RequestLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		slog.Info("http",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"duration", time.Since(start).String(),
			"ip", c.ClientIP(),
		)
	}
}

// OriginGuard aplica a proteção CSRF da ARQUITETURA §4.1: requisições que
// alteram estado precisam ser JSON e, quando trazem Origin, a origem deve
// ser a do sistema. Combinado com SameSite=Lax, fecha a superfície clássica.
func OriginGuard(allowedOrigin string) gin.HandlerFunc {
	origin := strings.TrimSuffix(strings.TrimSpace(allowedOrigin), "/")
	stateChanging := map[string]bool{
		http.MethodPost:   true,
		http.MethodPut:    true,
		http.MethodPatch:  true,
		http.MethodDelete: true,
	}
	return func(c *gin.Context) {
		if !stateChanging[c.Request.Method] {
			c.Next()
			return
		}
		if ct := c.ContentType(); ct != "application/json" {
			platform.AbortWithError(c, platform.NewAPIError(
				http.StatusUnsupportedMediaType, platform.CodeWrongTimestamp,
				"Content-Type deve ser application/json."))
			return
		}
		reqOrigin := c.GetHeader("Origin")
		if reqOrigin != "" && !strings.EqualFold(strings.TrimSuffix(reqOrigin, "/"), origin) {
			platform.AbortWithError(c, platform.NewAPIError(
				http.StatusForbidden, platform.CodeWrongTimestamp,
				"Origem da requisição não autorizada."))
			return
		}
		c.Next()
	}
}

// clientIP devolve o IP do cliente atrás do proxy reverso (Caddy) quando
// a requisição é confiável, senão o IP direto da conexão.
func clientIP(c *gin.Context) string {
	if c.Request.TLS == nil {
		if fwd := c.GetHeader("X-Forwarded-For"); fwd != "" {
			if host, _, err := net.SplitHostPort(strings.TrimSpace(strings.Split(fwd, ",")[0])); err == nil {
				return host
			}
			return strings.TrimSpace(strings.Split(fwd, ",")[0])
		}
	}
	return c.ClientIP()
}
