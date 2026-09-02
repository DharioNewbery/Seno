package handlers

import (
	"net/http"

	"seno/internal/middleware"
	"seno/internal/models"
	"seno/internal/services"
	"seno/pkg/response"
)

type AuthHandler struct {
	authService *services.AuthService
}

func NewAuthHandler(authService *services.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := decodeJSON(r, &req); err != nil {
		response.JSON(w, http.StatusBadRequest, response.Error("JSON inválido: "+err.Error()))
		return
	}

	result, err := h.authService.Register(r.Context(), services.RegisterInput{
		FullName: req.FullName,
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		status, msg := mapError(err)
		response.JSON(w, status, response.Error(msg))
		return
	}

	response.Created(w, "Usuário cadastrado com sucesso", toUserResponse(result.User))
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeJSON(r, &req); err != nil {
		response.JSON(w, http.StatusBadRequest, response.Error("JSON inválido: "+err.Error()))
		return
	}

	result, err := h.authService.Login(r.Context(), services.LoginInput{
		Login:    req.Login,
		Password: req.Password,
	})
	if err != nil {
		status, msg := mapError(err)
		response.JSON(w, status, response.Error(msg))
		return
	}

	roles := result.Roles
	if roles == nil {
		roles = []string{}
	}

	resp := loginResponse{
		User:  toUserResponse(result.User),
		Roles: roles,
		Tokens: tokenResponse{
			AccessToken:      result.Tokens.AccessToken,
			RefreshToken:     result.Tokens.RefreshToken,
			TokenType:        "Bearer",
			AccessExpiresAt:  result.Tokens.AccessExpiresAt,
			RefreshExpiresAt: result.Tokens.RefreshExpiresAt,
		},
	}

	response.OK(w, "Login realizado com sucesso", resp)
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := decodeJSON(r, &req); err != nil {
		response.JSON(w, http.StatusBadRequest, response.Error("JSON inválido: "+err.Error()))
		return
	}

	tokens, err := h.authService.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		status, msg := mapError(err)
		response.JSON(w, status, response.Error(msg))
		return
	}

	resp := tokenResponse{
		AccessToken:      tokens.AccessToken,
		RefreshToken:     tokens.RefreshToken,
		TokenType:        "Bearer",
		AccessExpiresAt:  tokens.AccessExpiresAt,
		RefreshExpiresAt: tokens.RefreshExpiresAt,
	}

	response.OK(w, "Token atualizado com sucesso", resp)
}

func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.JSON(w, http.StatusUnauthorized, response.Error("Autenticação necessária"))
		return
	}

	var req changePasswordRequest
	if err := decodeJSON(r, &req); err != nil {
		response.JSON(w, http.StatusBadRequest, response.Error("JSON inválido: "+err.Error()))
		return
	}

	err := h.authService.ChangePassword(r.Context(), services.ChangePasswordInput{
		UserID:          userID,
		CurrentPassword: req.CurrentPassword,
		NewPassword:     req.NewPassword,
	})
	if err != nil {
		status, msg := mapError(err)
		response.JSON(w, status, response.Error(msg))
		return
	}

	response.OK(w, "Senha alterada com sucesso", nil)
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.JSON(w, http.StatusUnauthorized, response.Error("Autenticação necessária"))
		return
	}

	result, err := h.authService.GetMe(r.Context(), userID)
	if err != nil {
		status, msg := mapError(err)
		response.JSON(w, status, response.Error(msg))
		return
	}

	roles := result.Roles
	if roles == nil {
		roles = []models.Role{}
	}

	response.OK(w, "Usuário autenticado", meResponse{
		User:  toUserResponse(result.User),
		Roles: roles,
	})
}
