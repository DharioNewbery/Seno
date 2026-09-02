package models

import (
	"time"

	"github.com/google/uuid"
)

// Assignment é uma tarefa de programação publicada numa turma.
type Assignment struct {
	ID        uuid.UUID  `db:"id" json:"id"`
	ClassID   uuid.UUID  `db:"class_id" json:"class_id"`
	Title     string     `db:"title" json:"title"`
	Statement string     `db:"statement" json:"statement"`
	Language  string     `db:"language" json:"language"`
	DueAt     *time.Time `db:"due_at" json:"due_at,omitempty"`
	CreatedAt time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt time.Time  `db:"updated_at" json:"updated_at"`
}

// AssignmentSummary é a tarefa nas listagens, com o nome da turma.
type AssignmentSummary struct {
	ID        uuid.UUID  `db:"id" json:"id"`
	ClassID   uuid.UUID  `db:"class_id" json:"class_id"`
	ClassName string     `db:"class_name" json:"class_name"`
	Title     string     `db:"title" json:"title"`
	Language  string     `db:"language" json:"language"`
	DueAt     *time.Time `db:"due_at" json:"due_at,omitempty"`
	CreatedAt time.Time  `db:"created_at" json:"created_at"`
}

// AssignmentDetail é a tarefa completa (enunciado) com as submissões visíveis
// ao solicitante: as próprias (aluno) ou todas (professor dono da turma).
type AssignmentDetail struct {
	ID          uuid.UUID        `db:"id" json:"id"`
	ClassID     uuid.UUID        `db:"class_id" json:"class_id"`
	ClassName   string           `db:"class_name" json:"class_name"`
	Title       string           `db:"title" json:"title"`
	Statement   string           `db:"statement" json:"statement"`
	Language    string           `db:"language" json:"language"`
	DueAt       *time.Time       `db:"due_at" json:"due_at,omitempty"`
	CreatedAt   time.Time        `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time        `db:"updated_at" json:"updated_at"`
	Submissions []SubmissionView `db:"-" json:"submissions"`
}

// Submission é o código entregue por um aluno numa tarefa.
type Submission struct {
	ID            uuid.UUID `db:"id" json:"id"`
	AssignmentID  uuid.UUID `db:"assignment_id" json:"assignment_id"`
	StudentUserID uuid.UUID `db:"student_user_id" json:"student_user_id"`
	Language      string    `db:"language" json:"language"`
	SourceCode    string    `db:"source_code" json:"source_code"`
	Status        string    `db:"status" json:"status"`
	CreatedAt     time.Time `db:"created_at" json:"created_at"`
}

// SubmissionView é a submissão nas listagens, com o nome do aluno.
type SubmissionView struct {
	ID            uuid.UUID `db:"id" json:"id"`
	StudentUserID uuid.UUID `db:"student_user_id" json:"student_user_id"`
	StudentName   string    `db:"student_name" json:"student_name"`
	Language      string    `db:"language" json:"language"`
	SourceCode    string    `db:"source_code" json:"source_code"`
	Status        string    `db:"status" json:"status"`
	CreatedAt     time.Time `db:"created_at" json:"created_at"`
}
