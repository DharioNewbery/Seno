package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"

	"seno/internal/models"
)

type userResponse struct {
	ID        uuid.UUID `json:"id"`
	FullName  string    `json:"full_name"`
	Email     string    `json:"email"`
	Username  *string   `json:"username,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

func toUserResponse(u models.User) userResponse {
	return userResponse{
		ID:        u.ID,
		FullName:  u.FullName,
		Email:     u.Email,
		Username:  u.Username,
		CreatedAt: u.CreatedAt,
	}
}

type tokenResponse struct {
	AccessToken      string    `json:"access_token"`
	RefreshToken     string    `json:"refresh_token"`
	TokenType        string    `json:"token_type"`
	AccessExpiresAt  time.Time `json:"access_expires_at"`
	RefreshExpiresAt time.Time `json:"refresh_expires_at"`
}

type registerRequest struct {
	FullName string `json:"full_name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

type createProfessorRequest struct {
	FullName string `json:"full_name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type createClassRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type joinClassRequest struct {
	Code string `json:"code"`
}

type createAssignmentRequest struct {
	Title     string  `json:"title"`
	Statement string  `json:"statement"`
	Language  string  `json:"language"`
	DueAt     *string `json:"due_at"`
}

type submitRequest struct {
	SourceCode string `json:"source_code"`
}

type saveDraftRequest struct {
	SourceCode string `json:"source_code"`
}

type assignmentResponse struct {
	ID        uuid.UUID  `json:"id"`
	ClassID   uuid.UUID  `json:"class_id"`
	Title     string     `json:"title"`
	Statement string     `json:"statement"`
	Language  string     `json:"language"`
	DueAt     *time.Time `json:"due_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

func toAssignmentResponse(a models.Assignment) assignmentResponse {
	return assignmentResponse{
		ID:        a.ID,
		ClassID:   a.ClassID,
		Title:     a.Title,
		Statement: a.Statement,
		Language:  a.Language,
		DueAt:     a.DueAt,
		CreatedAt: a.CreatedAt,
	}
}

type assignmentSummaryResponse struct {
	ID        uuid.UUID  `json:"id"`
	ClassID   uuid.UUID  `json:"class_id"`
	ClassName string     `json:"class_name"`
	Title     string     `json:"title"`
	Language  string     `json:"language"`
	DueAt     *time.Time `json:"due_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

func toAssignmentSummaryResponse(a models.AssignmentSummary) assignmentSummaryResponse {
	return assignmentSummaryResponse{
		ID:        a.ID,
		ClassID:   a.ClassID,
		ClassName: a.ClassName,
		Title:     a.Title,
		Language:  a.Language,
		DueAt:     a.DueAt,
		CreatedAt: a.CreatedAt,
	}
}

type assignmentDetailResponse struct {
	ID          uuid.UUID                   `json:"id"`
	ClassID     uuid.UUID                   `json:"class_id"`
	ClassName   string                      `json:"class_name"`
	Title       string                      `json:"title"`
	Statement   string                      `json:"statement"`
	Language    string                      `json:"language"`
	DueAt       *time.Time                  `json:"due_at,omitempty"`
	CreatedAt   time.Time                   `json:"created_at"`
	Submissions []submissionSummaryResponse `json:"submissions"`
	Draft       *models.Draft               `json:"draft,omitempty"`
}

func toAssignmentDetailResponse(d models.AssignmentDetail) assignmentDetailResponse {
	resp := assignmentDetailResponse{
		ID:          d.ID,
		ClassID:     d.ClassID,
		ClassName:   d.ClassName,
		Title:       d.Title,
		Statement:   d.Statement,
		Language:    d.Language,
		DueAt:       d.DueAt,
		CreatedAt:   d.CreatedAt,
		Submissions: make([]submissionSummaryResponse, 0, len(d.Submissions)),
		Draft:       d.Draft,
	}
	for _, s := range d.Submissions {
		resp.Submissions = append(resp.Submissions, toSubmissionSummaryResponse(s))
	}
	return resp
}

type submissionResponse struct {
	ID           uuid.UUID `json:"id"`
	AssignmentID uuid.UUID `json:"assignment_id"`
	Language     string    `json:"language"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
}

func toSubmissionResponse(s models.Submission) submissionResponse {
	return submissionResponse{
		ID:           s.ID,
		AssignmentID: s.AssignmentID,
		Language:     s.Language,
		Status:       s.Status,
		CreatedAt:    s.CreatedAt,
	}
}

type submissionSummaryResponse struct {
	ID          uuid.UUID `json:"id"`
	StudentName string    `json:"student_name"`
	Language    string    `json:"language"`
	SourceCode  string    `json:"source_code"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}

func toSubmissionSummaryResponse(s models.SubmissionView) submissionSummaryResponse {
	return submissionSummaryResponse{
		ID:          s.ID,
		StudentName: s.StudentName,
		Language:    s.Language,
		SourceCode:  s.SourceCode,
		Status:      s.Status,
		CreatedAt:   s.CreatedAt,
	}
}

type classResponse struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	JoinCode    string    `json:"join_code"`
	CreatedAt   time.Time `json:"created_at"`
}

func toClassResponse(c models.Class) classResponse {
	return classResponse{
		ID:          c.ID,
		Name:        c.Name,
		Description: c.Description,
		JoinCode:    c.JoinCode,
		CreatedAt:   c.CreatedAt,
	}
}

type classSummaryResponse struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	JoinCode    string    `json:"join_code"`
	MemberCount int       `json:"member_count"`
	CreatedAt   time.Time `json:"created_at"`
}

func toClassSummaryResponse(c models.ClassSummary) classSummaryResponse {
	return classSummaryResponse{
		ID:          c.ID,
		Name:        c.Name,
		Description: c.Description,
		JoinCode:    c.JoinCode,
		MemberCount: c.MemberCount,
		CreatedAt:   c.CreatedAt,
	}
}

type studentClassResponse struct {
	ID            uuid.UUID `json:"id"`
	Name          string    `json:"name"`
	Description   *string   `json:"description,omitempty"`
	ProfessorName string    `json:"professor_name"`
	JoinedAt      time.Time `json:"joined_at"`
}

func toStudentClassResponse(c models.StudentClass) studentClassResponse {
	return studentClassResponse{
		ID:            c.ID,
		Name:          c.Name,
		Description:   c.Description,
		ProfessorName: c.ProfessorName,
		JoinedAt:      c.JoinedAt,
	}
}

type professorResponse struct {
	UserID    uuid.UUID `json:"user_id"`
	FullName  string    `json:"full_name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

func toProfessorResponse(p models.ProfessorUser) professorResponse {
	return professorResponse{
		UserID:    p.UserID,
		FullName:  p.FullName,
		Email:     p.Email,
		CreatedAt: p.CreatedAt,
	}
}

type loginResponse struct {
	User   userResponse  `json:"user"`
	Roles  []string      `json:"roles"`
	Tokens tokenResponse `json:"tokens"`
}

type meResponse struct {
	User  userResponse  `json:"user"`
	Roles []models.Role `json:"roles"`
}

func decodeJSON(r *http.Request, dst interface{}) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}
