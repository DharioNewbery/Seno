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
	CreatedAt time.Time `json:"created_at"`
}

func toUserResponse(u models.User) userResponse {
	return userResponse{
		ID:        u.ID,
		FullName:  u.FullName,
		Email:     u.Email,
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
	Email    string `json:"email"`
	Password string `json:"password"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
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
