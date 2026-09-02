package handlers

import (
	"net/http"

	"seno/internal/services"
	"seno/pkg/response"
)

type ProfessorHandler struct {
	professorService *services.ProfessorService
}

func NewProfessorHandler(professorService *services.ProfessorService) *ProfessorHandler {
	return &ProfessorHandler{professorService: professorService}
}

func (h *ProfessorHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createProfessorRequest
	if err := decodeJSON(r, &req); err != nil {
		response.JSON(w, http.StatusBadRequest, response.Error("JSON inválido: "+err.Error()))
		return
	}

	user, err := h.professorService.CreateProfessor(r.Context(), services.CreateProfessorInput{
		FullName: req.FullName,
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		status, msg := mapError(err)
		response.JSON(w, status, response.Error(msg))
		return
	}

	response.Created(w, "Professor cadastrado com sucesso", toUserResponse(*user))
}

func (h *ProfessorHandler) List(w http.ResponseWriter, r *http.Request) {
	professors, err := h.professorService.List(r.Context())
	if err != nil {
		status, msg := mapError(err)
		response.JSON(w, status, response.Error(msg))
		return
	}

	resp := make([]professorResponse, 0, len(professors))
	for _, p := range professors {
		resp = append(resp, toProfessorResponse(p))
	}

	response.OK(w, "Professores listados com sucesso", resp)
}
