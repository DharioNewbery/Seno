package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"seno/internal/middleware"
	"seno/internal/services"
	"seno/pkg/response"
)

type AssignmentHandler struct {
	assignmentService *services.AssignmentService
}

func NewAssignmentHandler(assignmentService *services.AssignmentService) *AssignmentHandler {
	return &AssignmentHandler{assignmentService: assignmentService}
}

// parseDueAt converte a data de prazo (RFC3339, opcional) do corpo da requisição.
func parseDueAt(raw *string) (*time.Time, *services.ValidationError) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(*raw))
	if err != nil {
		return nil, services.NewValidationError("data de prazo inválida")
	}
	return &t, nil
}

func (h *AssignmentHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.JSON(w, http.StatusUnauthorized, response.Error("Autenticação necessária"))
		return
	}

	classID, err := uuid.Parse(chi.URLParam(r, "classID"))
	if err != nil {
		response.JSON(w, http.StatusBadRequest, response.Error("ID de turma inválido"))
		return
	}

	var req createAssignmentRequest
	if err := decodeJSON(r, &req); err != nil {
		response.JSON(w, http.StatusBadRequest, response.Error("JSON inválido: "+err.Error()))
		return
	}

	dueAt, vErr := parseDueAt(req.DueAt)
	if vErr != nil {
		response.JSON(w, http.StatusBadRequest, response.Error(vErr.Message))
		return
	}

	assignment, err := h.assignmentService.CreateAssignment(r.Context(), services.CreateAssignmentInput{
		ProfessorUserID: userID,
		ClassID:         classID,
		Title:           req.Title,
		Statement:       req.Statement,
		Language:        req.Language,
		DueAt:           dueAt,
	})
	if err != nil {
		status, msg := mapError(err)
		response.JSON(w, status, response.Error(msg))
		return
	}

	response.Created(w, "Tarefa criada com sucesso", toAssignmentResponse(*assignment))
}

func (h *AssignmentHandler) ListByClass(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.JSON(w, http.StatusUnauthorized, response.Error("Autenticação necessária"))
		return
	}

	classID, err := uuid.Parse(chi.URLParam(r, "classID"))
	if err != nil {
		response.JSON(w, http.StatusBadRequest, response.Error("ID de turma inválido"))
		return
	}

	assignments, err := h.assignmentService.ListByClass(r.Context(), userID, classID)
	if err != nil {
		status, msg := mapError(err)
		response.JSON(w, status, response.Error(msg))
		return
	}

	resp := make([]assignmentSummaryResponse, 0, len(assignments))
	for _, a := range assignments {
		resp = append(resp, toAssignmentSummaryResponse(a))
	}

	response.OK(w, "Tarefas listadas com sucesso", resp)
}

func (h *AssignmentHandler) Mine(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.JSON(w, http.StatusUnauthorized, response.Error("Autenticação necessária"))
		return
	}

	assignments, err := h.assignmentService.ListMine(r.Context(), userID)
	if err != nil {
		status, msg := mapError(err)
		response.JSON(w, status, response.Error(msg))
		return
	}

	resp := make([]assignmentSummaryResponse, 0, len(assignments))
	for _, a := range assignments {
		resp = append(resp, toAssignmentSummaryResponse(a))
	}

	response.OK(w, "Tarefas listadas com sucesso", resp)
}

func (h *AssignmentHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.JSON(w, http.StatusUnauthorized, response.Error("Autenticação necessária"))
		return
	}

	assignmentID, err := uuid.Parse(chi.URLParam(r, "assignmentID"))
	if err != nil {
		response.JSON(w, http.StatusBadRequest, response.Error("ID de tarefa inválido"))
		return
	}

	detail, err := h.assignmentService.GetDetail(r.Context(), userID, assignmentID)
	if err != nil {
		status, msg := mapError(err)
		response.JSON(w, status, response.Error(msg))
		return
	}

	response.OK(w, "Tarefa encontrada", toAssignmentDetailResponse(*detail))
}

// SaveDraft grava o rascunho do editor online (backup em tempo real).
func (h *AssignmentHandler) SaveDraft(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.JSON(w, http.StatusUnauthorized, response.Error("Autenticação necessária"))
		return
	}

	assignmentID, err := uuid.Parse(chi.URLParam(r, "assignmentID"))
	if err != nil {
		response.JSON(w, http.StatusBadRequest, response.Error("ID de tarefa inválido"))
		return
	}

	var req saveDraftRequest
	if err := decodeJSON(r, &req); err != nil {
		response.JSON(w, http.StatusBadRequest, response.Error("JSON inválido: "+err.Error()))
		return
	}

	draft, err := h.assignmentService.SaveDraft(r.Context(), services.SaveDraftInput{
		AssignmentID:  assignmentID,
		StudentUserID: userID,
		SourceCode:    req.SourceCode,
	})
	if err != nil {
		status, msg := mapError(err)
		response.JSON(w, status, response.Error(msg))
		return
	}

	response.OK(w, "Rascunho salvo", draft)
}

func (h *AssignmentHandler) Submit(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.JSON(w, http.StatusUnauthorized, response.Error("Autenticação necessária"))
		return
	}

	assignmentID, err := uuid.Parse(chi.URLParam(r, "assignmentID"))
	if err != nil {
		response.JSON(w, http.StatusBadRequest, response.Error("ID de tarefa inválido"))
		return
	}

	var req submitRequest
	if err := decodeJSON(r, &req); err != nil {
		response.JSON(w, http.StatusBadRequest, response.Error("JSON inválido: "+err.Error()))
		return
	}

	submission, err := h.assignmentService.Submit(r.Context(), services.SubmitInput{
		AssignmentID:  assignmentID,
		StudentUserID: userID,
		SourceCode:    req.SourceCode,
	})
	if err != nil {
		status, msg := mapError(err)
		response.JSON(w, status, response.Error(msg))
		return
	}

	response.Created(w, "Submissão enviada com sucesso", toSubmissionResponse(*submission))
}
