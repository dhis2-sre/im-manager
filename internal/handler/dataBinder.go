package handler

import (
	"github.com/dhis2-sre/im-manager/internal/errdef"

	"github.com/gin-gonic/gin"
)

func DataBinder(c *gin.Context, req interface{}) error {
	if c.ContentType() != "application/json" && c.ContentType() != "multipart/form-data" {
		return errdef.NewBadRequest("%s only accepts content of type application/json or multipart/form-data", c.FullPath())
	}

	if err := c.ShouldBind(req); err != nil {
		return errdef.NewBadRequest("Error binding data: %+v", err)
	}

	return nil
}
