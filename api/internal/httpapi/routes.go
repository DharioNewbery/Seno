// Package httpapi concentra as rotas Gin, handlers e middlewares
// (autenticação, RBAC) da API, conforme ARQUITETURA §3.
package httpapi

import (
	"github.com/gin-gonic/gin"

	"github.com/seno-project/seno/api/internal/domain"
)

// RegisterRoutes monta o roteador da API com o envelope de erros, as rotas
// da fundação e as rotas de autenticação (ARQUITETURA §5).
func RegisterRoutes(r *gin.Engine, deps *Dependencies) {
	r.Use(RequestLog(), OriginGuard(deps.Cfg.WebOrigin))

	r.GET("/healthz", deps.Healthz)

	v1 := r.Group("/v1")
	v1.GET("/healthz", deps.Healthz)

	auth := v1.Group("/auth")
	auth.POST("/login", deps.Login)
	auth.POST("/invite/accept", deps.AcceptInvite)
	auth.POST("/password/reset-request", deps.RequestPasswordReset)
	auth.POST("/password/reset", deps.ResetPassword)

	autenticado := auth.Group("")
	autenticado.Use(deps.RequireAuth())
	autenticado.POST("/logout", deps.Logout)
	autenticado.GET("/me", deps.Me)

	// Gestão de professores e admins (portal admin; alunos junto com
	// turmas/matrículas, em outro módulo).
	gestao := v1.Group("/users")
	gestao.Use(deps.RequireAuth(), deps.RequireCargo(domain.RoleAdmin, domain.RoleSuper))
	gestao.GET("", deps.ListarUsuarios)
	gestao.POST("", deps.CriarUsuario)
	gestao.PATCH("/:id", deps.AtualizarUsuario)
	gestao.POST("/:id/desativar", deps.BloquearUsuario)
	gestao.POST("/:id/ativar", deps.DesbloquearUsuario)
	gestao.POST("/:id/resetar-senha", deps.ResetarSenhaUsuario)

	// Ensino: matérias, períodos, turmas e matrículas (PROJETO §Organização).
	materias := v1.Group("/materias")
	materias.Use(
		deps.RequireAuth(),
		deps.RequireCargo(domain.RoleProfessor, domain.RoleAdmin, domain.RoleSuper),
	)
	materias.GET("", deps.ListarMaterias)
	escrita := materias.Group("")
	escrita.Use(deps.RequireCargo(domain.RoleAdmin, domain.RoleSuper))
	escrita.POST("", deps.CriarMateria)
	escrita.PATCH("/:id", deps.EditarMateria)
	escrita.DELETE("/:id", deps.ExcluirMateria)

	periodos := v1.Group("/periodos")
	periodos.Use(
		deps.RequireAuth(),
		deps.RequireCargo(domain.RoleProfessor, domain.RoleAdmin, domain.RoleSuper),
	)
	periodos.GET("", deps.ListarPeriodos)
	escritaPer := periodos.Group("")
	escritaPer.Use(deps.RequireCargo(domain.RoleAdmin, domain.RoleSuper))
	escritaPer.POST("", deps.CriarPeriodo)
	escritaPer.DELETE("/:id", deps.ExcluirPeriodo)

	turmas := v1.Group("/turmas")
	turmas.Use(
		deps.RequireAuth(),
		deps.RequireCargo(domain.RoleProfessor, domain.RoleAdmin, domain.RoleSuper),
	)
	turmas.GET("", deps.ListarTurmas)
	turmas.POST("", deps.CriaTurma)
	turmas.GET("/:id", deps.VerTurma)
	turmas.PATCH("/:id", deps.EditarTurma)
	turmas.POST("/:id/encerrar", deps.EncerrarTurma)
	turmas.DELETE("/:id", deps.ExcluirTurma)
	turmas.GET("/:id/matriculas", deps.ListarMatriculasTurma)
	turmas.POST("/:id/matriculas", deps.MatricularTurma)

	matriculas := v1.Group("/matriculas")
	matriculas.Use(
		deps.RequireAuth(),
		deps.RequireCargo(domain.RoleProfessor, domain.RoleAdmin, domain.RoleSuper),
	)
	matriculas.DELETE("/:id", deps.EncerrarMatricula)

	// Busca de alunos para a matrícula (professor+staff).
	alunos := v1.Group("/alunos")
	alunos.Use(
		deps.RequireAuth(),
		deps.RequireCargo(domain.RoleProfessor, domain.RoleAdmin, domain.RoleSuper),
	)
	alunos.GET("", deps.ListarAlunos)
}
