package middleware

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/dhis2-sre/im-manager/pkg/model"

	"github.com/dhis2-sre/im-manager/internal/errdef"
	"github.com/dhis2-sre/im-manager/internal/handler"
	"github.com/gin-gonic/gin"
)

func NewAuthorization(logger *slog.Logger, userService userService) AuthorizationMiddleware {
	return AuthorizationMiddleware{
		logger:      logger,
		userService: userService,
	}
}

type AuthorizationMiddleware struct {
	logger      *slog.Logger
	userService userService
}

type userService interface {
	FindById(id uint) (*model.User, error)
}

func (m AuthorizationMiddleware) RequireAdministrator(c *gin.Context) {
	u, err := handler.GetUserFromContext(c)
	if err != nil {
		return
	}

	userWithGroups, err := m.userService.FindById(u.ID)
	if err != nil {
		if errdef.IsNotFound(err) {
			_ = c.AbortWithError(http.StatusUnauthorized, err)
		} else {
			_ = c.Error(err)
		}
		return
	}

	if !userWithGroups.IsAdministrator() {
		// TODO investigate how to log user related info without logging sensitive information
		// failed requests are already logged by samber/slog-gin so we would want to add necessary
		// debug info into the middleware if possible
		m.logger.ErrorContext(c, "User tried to access administrator restricted endpoint", "user", u.ID)
		_ = c.AbortWithError(http.StatusUnauthorized, errors.New("administrator access denied"))
		return
	}

	// Extra precaution to ensure that no errors has occurred, and it's safe to call c.Next()
	if len(c.Errors.Errors()) > 0 {
		c.Abort()
		return
	} else {
		c.Next()
	}
}
