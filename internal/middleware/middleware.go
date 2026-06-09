package middleware

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// ErrorResponse is the standard error envelope for all API errors.
type ErrorResponse struct {
	Success bool       `json:"success"`
	Error   *APIError  `json:"error"`
}

type APIError struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

// SuccessResponse wraps successful data with optional pagination metadata.
type SuccessResponse struct {
	Success bool            `json:"success"`
	Data    interface{}     `json:"data"`
	Meta    *PaginationMeta `json:"meta,omitempty"`
}

type PaginationMeta struct {
	NextCursor string `json:"next_cursor,omitempty"`
	HasMore    bool   `json:"has_more"`
	Total      int64  `json:"total,omitempty"`
}

// Respond sends a consistent JSON success response.
func Respond(c *gin.Context, status int, data interface{}, meta *PaginationMeta) {
	c.JSON(status, SuccessResponse{
		Success: true,
		Data:    data,
		Meta:    meta,
	})
}

// RespondError sends a consistent JSON error response.
func RespondError(c *gin.Context, status int, code, message string, details interface{}) {
	c.JSON(status, ErrorResponse{
		Success: false,
		Error: &APIError{
			Code:    code,
			Message: message,
			Details: details,
		},
	})
}

// RequestLogger logs every incoming request with timing info.
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		duration := time.Since(start)
		log.Printf("%s %s %d %v", c.Request.Method, c.Request.URL.Path, c.Writer.Status(), duration)
	}
}

// Recovery catches panics and returns a 500 instead of crashing.
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("panic recovered: %v", err)
				RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "an unexpected error occurred", nil)
				c.Abort()
			}
		}()
		c.Next()
	}
}

// CORS allows cross-origin requests (needed for hosted demo + Swagger UI).
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
