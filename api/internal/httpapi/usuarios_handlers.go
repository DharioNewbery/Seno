package httpapi

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/seno-project/seno/api/internal/domain"
	"github.com/seno-project/seno/api/internal/platform"
	"github.com/seno-project/seno/api/internal/usuarios"
)

// Handlers da gestão de usuários do portal admin (apenas o super
// administra contas de admin — ARQUITETURA §5.4).

// ListarUsuarios: GET /v1/users?busca&status&cargo&pagina&por
func (d *Dependencies) ListarUsuarios(c *gin.Context) {
	user, _ := currentUser(c)
	f := usuarios.Filtros{
		Busca:  c.Query("busca"),
		Status: c.Query("status"),
		Cargo:  domain.Role(c.Query("cargo")),
	}
	if n, err := strconv.Atoi(c.Query("pagina")); err == nil {
		f.Pagina = n
	}
	if n, err := strconv.Atoi(c.Query("por")); err == nil {
		f.Por = n
	}
	pagina, err := d.UsuarioS.Listar(c.Request.Context(), user, f)
	if err != nil {
		platform.ErrorBody(c, err)
		return
	}
	c.JSON(http.StatusOK, pagina)
}

// criarPedido é o corpo do POST /v1/users — sem cargos: nascem depois,
// atribuídos na inspeção.
type criarPedido struct {
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

// CriarUsuario: POST /v1/users — nasce pendente com convite por e-mail.
func (d *Dependencies) CriarUsuario(c *gin.Context) {
	user, _ := currentUser(c)
	var req criarPedido
	if !bindJSON(c, &req) {
		return
	}
	criado, err := d.UsuarioS.Criar(c.Request.Context(), user, usuarios.Cadastro{
		Email:     req.Email,
		FirstName: req.FirstName,
		LastName:  req.LastName,
	})
	if err != nil {
		platform.ErrorBody(c, err)
		return
	}
	c.JSON(http.StatusCreated, criado)
}

// VerUsuario: GET /v1/users/:id — inspeção detalhada.
func (d *Dependencies) VerUsuario(c *gin.Context) {
	user, _ := currentUser(c)
	id, ok := idDaRota(c)
	if !ok {
		return
	}
	alvo, err := d.UsuarioS.Ver(c.Request.Context(), user, id)
	if err != nil {
		platform.ErrorBody(c, err)
		return
	}
	c.JSON(http.StatusOK, alvo)
}

// cargosPedido é o corpo do PUT /v1/users/:id/cargos.
type cargosPedido struct {
	Cargos []domain.Role `json:"cargos"`
}

// AlterarCargosUsuario: PUT /v1/users/:id/cargos — substitui os cargos.
func (d *Dependencies) AlterarCargosUsuario(c *gin.Context) {
	user, _ := currentUser(c)
	id, ok := idDaRota(c)
	if !ok {
		return
	}
	var req cargosPedido
	if !bindJSON(c, &req) {
		return
	}
	editado, err := d.UsuarioS.AlterarCargos(
		c.Request.Context(), user, id, req.Cargos)
	if err != nil {
		platform.ErrorBody(c, err)
		return
	}
	c.JSON(http.StatusOK, editado)
}

// editarPedido é o corpo do PATCH /v1/users/:id.
type editarPedido struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

// AtualizarUsuario: PATCH /v1/users/:id — nome/sobrenome da pessoa.
func (d *Dependencies) AtualizarUsuario(c *gin.Context) {
	user, _ := currentUser(c)
	id, ok := idDaRota(c)
	if !ok {
		return
	}
	var req editarPedido
	if !bindJSON(c, &req) {
		return
	}
	editado, err := d.UsuarioS.EditarDados(
		c.Request.Context(), user, id, req.FirstName, req.LastName)
	if err != nil {
		platform.ErrorBody(c, err)
		return
	}
	c.JSON(http.StatusOK, editado)
}

// BloquearUsuario: POST /v1/users/:id/desativar — 204; sessões morrem.
func (d *Dependencies) BloquearUsuario(c *gin.Context) {
	user, _ := currentUser(c)
	id, ok := idDaRota(c)
	if !ok {
		return
	}
	if err := d.UsuarioS.Bloquear(c.Request.Context(), user, id); err != nil {
		platform.ErrorBody(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// DesbloquearUsuario: POST /v1/users/:id/ativar — 204.
func (d *Dependencies) DesbloquearUsuario(c *gin.Context) {
	user, _ := currentUser(c)
	id, ok := idDaRota(c)
	if !ok {
		return
	}
	if err := d.UsuarioS.Desbloquear(c.Request.Context(), user, id); err != nil {
		platform.ErrorBody(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ResetarSenhaUsuario: POST /v1/users/:id/resetar-senha — envia e-mail de
// redefinição; sempre silencioso quanto à existência.
func (d *Dependencies) ResetarSenhaUsuario(c *gin.Context) {
	user, _ := currentUser(c)
	id, ok := idDaRota(c)
	if !ok {
		return
	}
	if err := d.UsuarioS.PedirNovaSenha(c.Request.Context(), user, id); err != nil {
		platform.ErrorBody(c, err)
		return
	}
	c.JSON(http.StatusAccepted, gin.H{
		"mensagem": "Se o usuário existir, o link de redefinição foi enviado.",
	})
}

// idDaRota converte :id; 422 quando inválido.
func idDaRota(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		platform.ErrorBody(c, platform.NewAPIError(
			http.StatusUnprocessableEntity, platform.CodeUnprocessable,
			"Identificador inválido."))
		return 0, false
	}
	return id, true
}
