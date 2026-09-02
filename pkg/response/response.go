package response

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// Body é a estrutura padrão de resposta da API.
type Body struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// JSON escreve uma resposta JSON com o status e payload informados.
func JSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// OK responde 200 com mensagem e dados opcionais.
func OK(w http.ResponseWriter, message string, data interface{}) {
	JSON(w, http.StatusOK, Body{Success: true, Message: message, Data: data})
}

// Created responde 201 com mensagem e dados opcionais.
func Created(w http.ResponseWriter, message string, data interface{}) {
	JSON(w, http.StatusCreated, Body{Success: true, Message: message, Data: data})
}

// Error monta o corpo de erro para uso com JSON.
func Error(message string) Body {
	return Body{Success: false, Error: message}
}

// Errorf monta o corpo de erro com formatação.
func Errorf(format string, args ...interface{}) Body {
	return Body{Success: false, Error: fmt.Sprintf(format, args...)}
}
