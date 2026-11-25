package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/proxlab/backend/internal/domain/entity"
	"github.com/proxlab/backend/internal/usecase"
)

type ProfileHandler struct {
	profileUC *usecase.ProfileUseCase
}

func NewProfileHandler(profileUC *usecase.ProfileUseCase) *ProfileHandler {
	return &ProfileHandler{profileUC: profileUC}
}

type updateProfileRequest struct {
	Name string `json:"name" binding:"max=120"`
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password"     binding:"required,min=8"`
}

func profileJSON(u *entity.User) gin.H {
	return gin.H{"id": u.ID, "email": u.Email, "name": u.Name, "role": u.Role, "created_at": u.CreatedAt}
}

// Get godoc
//
// GET /api/me
// 200: { "id": "uuid", "email": "joao@ifpe.edu.br", "name": "João", "role": "aluno", "created_at": "..." }
// 404: { "error": "usuário não encontrado" }
func (h *ProfileHandler) Get(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	user, err := h.profileUC.Get(c.Request.Context(), userID)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, profileJSON(user))
}

// Update godoc
//
// PATCH /api/me
// Body: { "name": "João Silva" }   (e-mail e perfil não são alteráveis)
// 200:  mesmo corpo de GET /api/me
func (h *ProfileHandler) Update(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	var req updateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	user, err := h.profileUC.UpdateName(c.Request.Context(), userID, req.Name)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, profileJSON(user))
}

// ChangePassword godoc
//
// PUT /api/me/password
// Body: { "current_password": "...", "new_password": "..." }
// 204:  senha trocada
// 403:  { "error": "senha atual incorreta" }  (não é 401 para o front não encerrar a sessão)
func (h *ProfileHandler) ChangePassword(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	var req changePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "informe a senha atual e uma nova senha com pelo menos 8 caracteres"})
		return
	}
	if err := h.profileUC.ChangePassword(c.Request.Context(), userID, req.CurrentPassword, req.NewPassword); err != nil {
		h.fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *ProfileHandler) fail(c *gin.Context, err error) {
	switch {
	case errors.Is(err, usecase.ErrUserNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, usecase.ErrWrongPassword):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	case errors.Is(err, usecase.ErrInvalidName), errors.Is(err, usecase.ErrWeakPassword):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno do servidor"})
	}
}
