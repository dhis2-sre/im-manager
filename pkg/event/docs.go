package event

import "github.com/gin-contrib/sse"

// swagger:response Stream
type StreamBody struct {
	// in: body
	Body sse.Event
}
