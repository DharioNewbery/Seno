package models

import (
	"time"

	"github.com/google/uuid"
)

// Class é uma turma criada por um professor; JoinCode é o código de ingresso.
type Class struct {
	ID              uuid.UUID `db:"id" json:"id"`
	Name            string    `db:"name" json:"name"`
	Description     *string   `db:"description" json:"description,omitempty"`
	JoinCode        string    `db:"join_code" json:"join_code"`
	ProfessorUserID uuid.UUID `db:"professor_user_id" json:"professor_user_id"`
	CreatedAt       time.Time `db:"created_at" json:"created_at"`
	UpdatedAt       time.Time `db:"updated_at" json:"updated_at"`
}

// ClassSummary é a turma na listagem do professor, com contagem de alunos.
type ClassSummary struct {
	ID          uuid.UUID `db:"id" json:"id"`
	Name        string    `db:"name" json:"name"`
	Description *string   `db:"description" json:"description,omitempty"`
	JoinCode    string    `db:"join_code" json:"join_code"`
	MemberCount int       `db:"member_count" json:"member_count"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
}

// StudentClass é a turma na listagem do aluno, com o professor e a data de ingresso.
type StudentClass struct {
	ID            uuid.UUID `db:"id" json:"id"`
	Name          string    `db:"name" json:"name"`
	Description   *string   `db:"description" json:"description,omitempty"`
	ProfessorName string    `db:"professor_name" json:"professor_name"`
	JoinedAt      time.Time `db:"joined_at" json:"joined_at"`
}
