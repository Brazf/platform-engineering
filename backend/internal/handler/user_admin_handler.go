package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/proxlab/backend/internal/usecase"
)

// UserAdminHandler: tela "Usuários cadastrados" (somente pesquisador, via RequireRole).
type UserAdminHandler struct {
	adminUC *usecase.UserAdminUseCase
}

func NewUserAdminHandler(adminUC *usecase.UserAdminUseCase) *UserAdminHandler {
	return &UserAdminHandler{adminUC: adminUC}
}

type updateEmailRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// List godoc
//
// GET /api/users
// 200: [ { "id", "email", "name", "role", "created_at", "vm_count": 2, "container_count": 1 } ]
func (h *UserAdminHandler) List(c *gin.Context) {
	items, err := h.adminUC.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno do servidor"})
		return
	}
	out := make([]gin.H, 0, len(items))
	for _, it := range items {
		u := it.User
		out = append(out, gin.H{
			"id": u.ID, "email": u.Email, "name": u.Name, "role": u.Role, "created_at": u.CreatedAt,
			"vm_count": it.VMCount, "container_count": it.ContainerCount,
		})
	}
	c.JSON(http.StatusOK, out)
}

// UpdateEmail godoc
//
// PATCH /api/users/:id
// Body: { "email": "novo@ifpe.edu.br" }   (único campo alterável pelo pesquisador)
// 200:  { "id", "email", "name", "role", "created_at" }
// 404 usuário não encontrado · 409 e-mail já está em uso
func (h *UserAdminHandler) UpdateEmail(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id inválido"})
		return
	}
	var req updateEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "informe um e-mail válido"})
		return
	}
	user, err := h.adminUC.UpdateEmail(c.Request.Context(), id, req.Email)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, profileJSON(user))
}

// Delete godoc
//
// DELETE /api/users/:id
// Apaga todas as VMs e containers do usuário e depois a conta.
// 204 excluído · 400 a própria conta · 404 não encontrado
// 503 { "error": "não foi possível apagar 1 VM(s) e 0 container(s)..." } — usuário mantido
func (h *UserAdminHandler) Delete(c *gin.Context) {
	actorID, ok := getUserID(c)
	if !ok {
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id inválido"})
		return
	}
	if err := h.adminUC.Delete(c.Request.Context(), actorID, id); err != nil {
		h.fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *UserAdminHandler) fail(c *gin.Context, err error) {
	var remaining *usecase.ResourcesRemainingError
	switch {
	case errors.As(err, &remaining):
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
	case errors.Is(err, usecase.ErrUserNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, usecase.ErrEmailAlreadyInUse):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, usecase.ErrCannotDeleteSelf):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno do servidor"})
	}
}
