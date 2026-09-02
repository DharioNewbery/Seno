package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"seno/internal/services"
	"seno/pkg/response"
)

type UserHandler struct {
	userService *services.UserService
}

func NewUserHandler(userService *services.UserService) *UserHandler {
	return &UserHandler{userService: userService}
}

func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	users, err := h.userService.List(r.Context())
	if err != nil {
		status, msg := mapError(err)
		response.JSON(w, status, response.Error(msg))
		return
	}

	resp := make([]userResponse, 0, len(users))
	for _, u := range users {
		resp = append(resp, toUserResponse(u))
	}

	response.OK(w, "Usuários listados com sucesso", resp)
}

func (h *UserHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		response.JSON(w, http.StatusBadRequest, response.Error("ID de usuário inválido"))
		return
	}

	user, err := h.userService.GetByID(r.Context(), id)
	if err != nil {
		status, msg := mapError(err)
		response.JSON(w, status, response.Error(msg))
		return
	}

	response.OK(w, "Usuário encontrado", toUserResponse(*user))
}
