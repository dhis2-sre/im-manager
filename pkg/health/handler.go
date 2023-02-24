package health

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Response struct {
	Status string `json:"status"`
}

// Health status
func Health(c *gin.Context) {
	// swagger:route GET /health health
	//
	// Health status
	//
	// Show service health status
	//
	// Responses:
	//   200: Response
	c.JSON(http.StatusOK, Response{"UP"})
}
