package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/seno-project/seno/api/internal/domain"
	"github.com/seno-project/seno/api/internal/platform"
)

// Handlers do domínio de autenticação (ARQUITETURA §5.2/§5.3).

// loginRequest é o corpo do POST /v1/auth/login.
type loginRequest struct {
	Email string `json:"email"`
	Senha string `json:"senha"`
}

// pessoaView é a projeção da pessoa no login (cargos vão separados,
// conforme o contrato {token, pessoa, cargos}).
type pessoaView struct {
	ID        int64  `json:"id"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

// loginResponse é a resposta do login.
type loginResponse struct {
	Token  string        `json:"token"`
	Pessoa pessoaView    `json:"pessoa"`
	Cargos []domain.Role `json:"cargos"`
}

// Login autentica credenciais e abre uma sessão opaca. Público.
func (d *Dependencies) Login(c *gin.Context) {
	var req loginRequest
	if !bindJSON(c, &req) {
		return
	}
	if req.Email == "" || req.Senha == "" {
		platform.ErrorBody(c, platform.NewAPIError(
			http.StatusUnprocessableEntity, platform.CodeUnprocessable,
			"Informe e-mail e senha."))
		return
	}
	user, token, _, err := d.Auth.Login(
		c.Request.Context(), req.Email, req.Senha, clientIP(c), c.GetHeader("User-Agent"))
	if err != nil {
		platform.ErrorBody(c, err)
		return
	}
	c.JSON(http.StatusOK, loginResponse{
		Token: token,
		Pessoa: pessoaView{
			ID:        user.ID,
			Email:     user.Email,
			FirstName: user.FirstName,
			LastName:  user.LastName,
		},
		Cargos: user.Roles,
	})
}

// Logout encerra a sessão corrente. Autenticado.
func (d *Dependencies) Logout(c *gin.Context) {
	user, _ := currentUser(c)
	if err := d.Auth.Logout(c.Request.Context(), currentToken(c), user); err != nil {
		platform.ErrorBody(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// Me devolve o usuário autenticado com cargos revalidados no banco.
func (d *Dependencies) Me(c *gin.Context) {
	user, _ := currentUser(c)
	c.JSON(http.StatusOK, user)
}

// resetRequestRequest é o corpo do POST /v1/auth/password/reset-request.
type resetRequestRequest struct {
	Email string `json:"email"`
}

// RequestPasswordReset inicia a recuperação de senha por e-mail. Público
// e silencioso: a resposta é igual exista ou não a conta (anti-enumeração).
func (d *Dependencies) RequestPasswordReset(c *gin.Context) {
	var req resetRequestRequest
	if !bindJSON(c, &req) {
		return
	}
	if req.Email == "" {
		platform.ErrorBody(c, platform.NewAPIError(
			http.StatusUnprocessableEntity, platform.CodeUnprocessable,
			"Informe o e-mail."))
		return
	}
	if err := d.Cargo.RequestPasswordReset(c.Request.Context(), req.Email); err != nil {
		platform.ErrorBody(c, err)
		return
	}
	c.JSON(http.StatusAccepted, gin.H{
		"mensagem": "Se o e-mail estiver cadastrado, enviaremos as instruções de redefinição.",
	})
}

// inviteRequest é o corpo do POST /v1/auth/invite/accept.
type inviteRequest struct {
	Token string `json:"token"`
	Senha string `json:"senha"`
}

// AcceptInvite consome o convite (uso único) e define a senha inicial,
// ativando o usuário pendente. Público: quem expira o convite não tem
// senha ainda.
func (d *Dependencies) AcceptInvite(c *gin.Context) {
	var req inviteRequest
	if !bindJSON(c, &req) {
		return
	}
	if req.Token == "" {
		platform.ErrorBody(c, platform.NewAPIError(
			http.StatusUnprocessableEntity, platform.CodeUnprocessable,
			"Token de convite ausente."))
		return
	}
	if req.Senha == "" {
		platform.ErrorBody(c, platform.NewAPIError(
			http.StatusUnprocessableEntity, platform.CodeUnprocessable,
			"Informe a senha."))
		return
	}
	if err := d.Cargo.AcceptInvite(c.Request.Context(), req.Token, req.Senha); err != nil {
		platform.ErrorBody(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"mensagem": "Senha definida. Você já pode entrar no sistema."})
}

// resetPasswordRequest é o corpo do POST /v1/auth/password/reset.
type resetPasswordRequest struct {
	Token string `json:"token"`
	Senha string `json:"senha"`
}

// ResetPassword consome o token de uso único e define a nova senha,
// revogando todas as sessões do usuário (ARQUITETURA §5.1/§5.3).
func (d *Dependencies) ResetPassword(c *gin.Context) {
	var req resetPasswordRequest
	if !bindJSON(c, &req) {
		return
	}
	if req.Token == "" {
		platform.ErrorBody(c, platform.NewAPIError(
			http.StatusUnprocessableEntity, platform.CodeUnprocessable,
			"Token de redefinição ausente."))
		return
	}
	if req.Senha == "" {
		platform.ErrorBody(c, platform.NewAPIError(
			http.StatusUnprocessableEntity, platform.CodeUnprocessable,
			"Informe a nova senha."))
		return
	}
	if err := d.Cargo.ResetPassword(c.Request.Context(), req.Token, req.Senha); err != nil {
		platform.ErrorBody(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"mensagem": "Senha redefinida."})
}
