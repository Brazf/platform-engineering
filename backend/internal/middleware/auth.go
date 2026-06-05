package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const (
	ContextKeyUserID = "userID"
	ContextKeyRole   = "role"
)

type jwtClaims struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// Auth valida o JWT do header Authorization: Bearer <token>.
// Injeta userID (string) e role (string) no contexto Gin.
//
// 401 — token ausente, inválido ou expirado.
func Auth(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Aceita token via header Authorization ou query param ?token= (para WebSocket)
		var rawToken string
		if authHeader := c.GetHeader("Authorization"); strings.HasPrefix(authHeader, "Bearer ") {
			rawToken = strings.TrimPrefix(authHeader, "Bearer ")
		} else if qToken := c.Query("token"); qToken != "" {
			rawToken = qToken
		} else {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token de autenticação ausente"})
			return
		}

		token, err := jwt.ParseWithClaims(rawToken, &jwtClaims{}, func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(jwtSecret), nil
		})
		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token inválido ou expirado"})
			return
		}

		claims, ok := token.Claims.(*jwtClaims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token com formato inválido"})
			return
		}

		c.Set(ContextKeyUserID, claims.UserID)
		c.Set(ContextKeyRole, claims.Role)
		c.Next()
	}
}

// RequireRole aborta com 403 se o role do usuário autenticado não estiver na lista permitida.
// Deve ser encadeado após o middleware Auth.
//
// Uso: vms.Use(middleware.RequireRole("aluno", "pesquisador"))
func RequireRole(roles ...string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		allowed[r] = struct{}{}
	}

	return func(c *gin.Context) {
		roleVal, exists := c.Get(ContextKeyRole)
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token de autenticação ausente"})
			return
		}

		role, ok := roleVal.(string)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token com formato inválido"})
			return
		}

		if _, permitted := allowed[role]; !permitted {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "acesso não autorizado para este perfil"})
			return
		}

		c.Next()
	}
}
