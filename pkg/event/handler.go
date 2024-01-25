package event

import (
	"io"
	"log"

	"github.com/dhis2-sre/im-manager/internal/handler"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/gin-gonic/gin"
)

func NewHandler(broker broker) Handler {
	return Handler{broker}
}

type Handler struct {
	broker broker
}

type broker interface {
	Subscribe(user model.User)
	Unsubscribe(id uint) error
	Receive(id uint) (Event, bool)
}

func (h Handler) Subscribe(c *gin.Context) {
	// swagger:route GET /subscribe streamSSE
	//
	// Stream events
	//
	// Stream events...
	//
	// responses:
	//   200: Stream
	//   400: Error
	//   404: Error
	//   415: Error
	//
	// security:
	//   oauth2:
	user, err := handler.GetUserFromContext(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	h.broker.Subscribe(*user)

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Transfer-Encoding", "chunked")

	defer func() {
		err := h.broker.Unsubscribe(user.ID)
		if err != nil {
			log.Println(err)
		}
		log.Printf("Closing client %d", user.ID)
	}()

	go func() {
		<-c.Done()
		err := h.broker.Unsubscribe(user.ID)
		if err != nil {
			log.Println(err)
		}
		log.Printf("Closing client %d", user.ID)
	}()

	c.Stream(func(w io.Writer) bool {
		if event, ok := h.broker.Receive(user.ID); ok {
			c.SSEvent(event.Type, event.Message)
			return true
		}
		return false
	})
}
