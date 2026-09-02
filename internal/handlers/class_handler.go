package handlers

import (
	"net/http"

	"seno/internal/middleware"
	"seno/internal/services"
	"seno/pkg/response"
)

type ClassHandler struct {
	classService *services.ClassService
}

func NewClassHandler(classService *services.ClassService) *ClassHandler {
	return &ClassHandler{classService: classService}
}

func (h *ClassHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.JSON(w, http.StatusUnauthorized, response.Error("Autenticação necessária"))
		return
	}

	var req createClassRequest
	if err := decodeJSON(r, &req); err != nil {
		response.JSON(w, http.StatusBadRequest, response.Error("JSON inválido: "+err.Error()))
		return
	}

	class, err := h.classService.CreateClass(r.Context(), services.CreateClassInput{
		ProfessorUserID: userID,
		Name:            req.Name,
		Description:     req.Description,
	})
	if err != nil {
		status, msg := mapError(err)
		response.JSON(w, status, response.Error(msg))
		return
	}

	response.Created(w, "Turma criada com sucesso", toClassResponse(*class))
}

func (h *ClassHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.JSON(w, http.StatusUnauthorized, response.Error("Autenticação necessária"))
		return
	}

	classes, err := h.classService.ListByProfessor(r.Context(), userID)
	if err != nil {
		status, msg := mapError(err)
		response.JSON(w, status, response.Error(msg))
		return
	}

	resp := make([]classSummaryResponse, 0, len(classes))
	for _, c := range classes {
		resp = append(resp, toClassSummaryResponse(c))
	}

	response.OK(w, "Turmas listadas com sucesso", resp)
}

func (h *ClassHandler) Join(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.JSON(w, http.StatusUnauthorized, response.Error("Autenticação necessária"))
		return
	}

	var req joinClassRequest
	if err := decodeJSON(r, &req); err != nil {
		response.JSON(w, http.StatusBadRequest, response.Error("JSON inválido: "+err.Error()))
		return
	}

	class, err := h.classService.JoinByCode(r.Context(), services.JoinClassInput{
		StudentUserID: userID,
		Code:          req.Code,
	})
	if err != nil {
		status, msg := mapError(err)
		response.JSON(w, status, response.Error(msg))
		return
	}

	response.OK(w, "Você ingressou na turma", toClassResponse(*class))
}

func (h *ClassHandler) Mine(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.JSON(w, http.StatusUnauthorized, response.Error("Autenticação necessária"))
		return
	}

	classes, err := h.classService.ListByStudent(r.Context(), userID)
	if err != nil {
		status, msg := mapError(err)
		response.JSON(w, status, response.Error(msg))
		return
	}

	resp := make([]studentClassResponse, 0, len(classes))
	for _, c := range classes {
		resp = append(resp, toStudentClassResponse(c))
	}

	response.OK(w, "Turmas listadas com sucesso", resp)
}
