package server

import (
	"net/http"
	"sort"

	"github.com/go-chi/chi/v5"

	"seno/pkg/response"
)

type routeInfo struct {
	Method string `json:"method"`
	Path   string `json:"path"`
}

// listRoutesHandler percorre o roteador (chi.Walk) e retorna todas as rotas
// registradas, ordenadas por caminho e método.
func listRoutesHandler(router chi.Routes) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		routes := []routeInfo{}

		_ = chi.Walk(router, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
			if route == "/routes" {
				return nil
			}
			routes = append(routes, routeInfo{Method: method, Path: route})
			return nil
		})

		sort.Slice(routes, func(i, j int) bool {
			if routes[i].Path != routes[j].Path {
				return routes[i].Path < routes[j].Path
			}
			return routes[i].Method < routes[j].Method
		})

		response.OK(w, "Rotas disponíveis", routes)
	}
}
