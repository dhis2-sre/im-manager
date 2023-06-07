package handler

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func GetPathParameter(c *gin.Context, parameter string) (uint, bool) {
	idParam := c.Param(parameter)
	id, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil {
		_ = c.AbortWithError(http.StatusBadRequest, fmt.Errorf("error parsing %q: %v", parameter, err))
		return 0, false
	}
	return uint(id), true
}
