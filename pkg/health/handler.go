package health

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

type Response struct {
	Status string `json:"status"`
}

// Health
// swagger:route GET /health health
//
// Service health status
//
// responses:
//   200: Response
func Health(c *gin.Context) {
	c.JSON(http.StatusOK, Response{"UP"})
}
