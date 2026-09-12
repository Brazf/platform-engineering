package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/proxlab/backend/internal/usecase"
)

type AuthHandler struct {
	authUC *usecase.AuthUseCase
}

func NewAuthHandler(authUC *usecase.AuthUseCase) *AuthHandler {
	return &AuthHandler{authUC: authUC}
}

type registerRequest struct {
	Email    string `json:"email"    binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
	Role     string `json:"role"     binding:"required,oneof=aluno pesquisador"`
}

type loginRequest struct {
	Email    string `json:"email"    binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// Register godoc
//
// POST /api/users  (exige Bearer de um usuário com role "pesquisador")
// Body:   { "email": "joao@ifpe.edu.br", "password": "senha1234", "role": "aluno" }
// 201:    { "id": "3fa85f64-...", "email": "joao@ifpe.edu.br", "role": "aluno" }
// 401/403: sem token ou perfil sem permissão (middleware)
// 400:    { "error": "Key: 'registerRequest.Email' Error:Field validation for 'Email' failed..." }
// 409:    { "error": "email já está em uso" }
// 422:    { "error": "perfil inválido: use 'aluno' ou 'pesquisador'" }
func (h *AuthHandler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := h.authUC.Register(req.Email, req.Password, req.Role)
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrEmailAlreadyInUse):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		case errors.Is(err, usecase.ErrInvalidRole):
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno do servidor"})
		}
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":    user.ID,
		"email": user.Email,
		"role":  user.Role,
	})
}

// Login godoc
//
// POST /api/auth/login
// Body:   { "email": "joao@ifpe.edu.br", "password": "senha1234" }
// 200:    { "token": "eyJ...", "user": { "id": "uuid", "email": "joao@ifpe.edu.br", "role": "aluno" } }
// 400:    { "error": "Key: 'loginRequest.Email' Error:..." }
// 401:    { "error": "email ou senha inválidos" }
func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	out, err := h.authUC.Login(req.Email, req.Password)
	if err != nil {
		if errors.Is(err, usecase.ErrInvalidCredentials) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno do servidor"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": out.Token,
		"user": gin.H{
			"id":    out.User.ID,
			"email": out.User.Email,
			"role":  out.User.Role,
		},
	})
}
