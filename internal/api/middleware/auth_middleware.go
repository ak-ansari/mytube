package middleware

import (
	// "net/http"
	// "strings"

	"github.com/ak-ansari/mytube/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	// "github.com/google/uuid"
)

var str = "49672439-457b-44af-9e9a-7760d4dee9e3"
var userId, _ = uuid.Parse(str)

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// authHeader := c.GetHeader("Authorization")
		// if authHeader == "" {
		//     c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing Authorization header"})
		//     return
		// }

		// parts := strings.SplitN(authHeader, " ", 2)
		// if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		//     c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid Authorization format"})
		//     return
		// }

		// tokenString := parts[1]

		// token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		//     // Validate signing algorithm
		//     if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
		//         return nil, jwt.ErrSignatureInvalid
		//     }
		//     return config.JWTSecret, nil
		// })

		// if err != nil || !token.Valid {
		//     c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
		//     return
		// }

		// claims, ok := token.Claims.(jwt.MapClaims)
		// if !ok {
		//     c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token claims"})
		//     return
		// }

		// Example: user_id embedded in token claims
		// userIDStr, ok := claims["user_id"].(string)
		// if !ok {
		//     c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing user_id in token"})
		//     return
		// }

		// userID, err := uuid.Parse(userIDStr)
		// if err != nil {
		//     c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid user_id format"})
		//     return
		// }

		// Attach user_id to context for use in handlers
		user := &models.User{
			UserName:    "abdul karim",
			ID:          userId,
			ChannelName: "ak-live",
			Avatar:      "",
			Status:      "active",
		}
		c.Set("user", user)

		// Continue
		c.Next()
	}
}
