package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// AuthMiddleware provides authentication middleware
type AuthMiddleware struct {
	jwtSecret []byte
	logger    *zap.Logger
}

// NewAuthMiddleware creates a new auth middleware
func NewAuthMiddleware(jwtSecret string, logger *zap.Logger) *AuthMiddleware {
	return &AuthMiddleware{
		jwtSecret: []byte(jwtSecret),
		logger:    logger,
	}
}

// RequireAuth requires authentication for the route
func (m *AuthMiddleware) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "Unauthorized",
				"message": "Missing authorization header",
			})
			return
		}

		// Extract token from "Bearer <token>"
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "Unauthorized",
				"message": "Invalid authorization header format",
			})
			return
		}

		token := parts[1]

		// TODO: Validate JWT token
		// claims, err := ValidateToken(token, m.jwtSecret)
		// if err != nil {
		// 	c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
		// 		"error":   "Unauthorized",
		// 		"message": "Invalid token",
		// 	})
		// 	return
		// }

		// Set user info in context
		// c.Set("user_id", claims.UserID)
		// c.Set("user_role", claims.Role)

		c.Next()
	}
}

// RequireRole requires a specific role for the route
func (m *AuthMiddleware) RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO: Get user role from context
		// userRole, exists := c.Get("user_role")
		// if !exists {
		// 	c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
		// 		"error":   "Unauthorized",
		// 		"message": "User role not found",
		// 	})
		// 	return
		// }

		// Check if user has required role
		// for _, role := range roles {
		// 	if userRole == role {
		// 		c.Next()
		// 		return
		// 	}
		// }

		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"error":   "Forbidden",
			"message": "Insufficient permissions",
		})
	}
}

// RateLimiter provides rate limiting middleware
type RateLimiter struct {
	limit  int
	window int // seconds
	logger *zap.Logger
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(limit int, window int, logger *zap.Logger) *RateLimiter {
	return &RateLimiter{
		limit:  limit,
		window: window,
		logger: logger,
	}
}

// Limit returns a rate limiting middleware
func (r *RateLimiter) Limit() gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO: Implement rate limiting using Redis
		// key := fmt.Sprintf("ratelimit:%s:%s", c.ClientIP(), c.FullPath())
		// allowed, err := redisClient.AcquireRateLimit(context.Background(), key, r.limit, time.Duration(r.window)*time.Second)
		// if err != nil {
		// 	r.logger.Error("Rate limit error", zap.Error(err))
		// 	c.Next() // Allow on error
		// 	return
		// }
		// if !allowed {
		// 	c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
		// 		"error":   "Too Many Requests",
		// 		"message": "Rate limit exceeded",
		// 	})
		// 	return
		// }

		c.Next()
	}
}

// ValidationMiddleware provides request validation middleware
type ValidationMiddleware struct {
	logger *zap.Logger
}

// NewValidationMiddleware creates a new validation middleware
func NewValidationMiddleware(logger *zap.Logger) *ValidationMiddleware {
	return &ValidationMiddleware{
		logger: logger,
	}
}

// ValidateJSON validates JSON request body
func (m *ValidationMiddleware) ValidateJSON() gin.HandlerFunc {
	return func(c *gin.Context) {
		contentType := c.GetHeader("Content-Type")
		if !strings.Contains(contentType, "application/json") {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error":   "Bad Request",
				"message": "Content-Type must be application/json",
			})
			return
		}

		c.Next()
	}
}
