package models

import (
	"time"

	"github.com/google/uuid"
)

// ProfessorUser carrega o vínculo de professor junto aos dados públicos do
// usuário (resultado de leitura com JOIN; a escrita usa models.User).
type ProfessorUser struct {
	UserID    uuid.UUID `db:"user_id" json:"user_id"`
	FullName  string    `db:"full_name" json:"full_name"`
	Email     string    `db:"email" json:"email"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}
