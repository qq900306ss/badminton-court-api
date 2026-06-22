package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/qq900306ss/badminton-court-api/internal/repository"
)

// Health reports liveness without touching DynamoDB. If table setup failed,
// the error is surfaced here so we can diagnose without CloudWatch access.
func Health(c *gin.Context) {
	dbErr := repository.LastTableError()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"service": "badminton-court-api",
		"db_setup_error": func() string {
			if dbErr == "" {
				return "none"
			}
			return dbErr
		}(),
	})
}

func ok(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

func fail(c *gin.Context, status int, msg string) {
	c.JSON(status, gin.H{"success": false, "error": msg})
}
