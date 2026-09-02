// Package web embute e serve os arquivos estáticos da interface do Seno.
package web

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed static
var staticFS embed.FS

// Handler serve a interface web (web/static) na raiz do servidor.
// O panic é aceitável: fs.Sub só falha se "static" não existir no embed,
// o que é garantido em tempo de compilação.
func Handler() http.Handler {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic("web: diretório static não encontrado no embed: " + err.Error())
	}
	return http.FileServer(http.FS(sub))
}
