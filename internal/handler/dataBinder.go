package handler

import (
	"fmt"

	"github.com/dhis2-sre/im-manager/internal/apperror"
	"github.com/gin-gonic/gin"
)

func DataBinder(c *gin.Context, req interface{}) error {
	if c.ContentType() != "application/json" && c.ContentType() != "multipart/form-data" {
		reason := fmt.Sprintf("%s only accepts content of type application/json or multipart/form-data", c.FullPath())
		return apperror.NewUnsupportedMediaType(reason)
	}

	if err := c.ShouldBind(req); err != nil {
		message := fmt.Sprintf("Error binding data: %+v\n", err)
		return apperror.NewBadRequest(message)
	}

	return nil
}
