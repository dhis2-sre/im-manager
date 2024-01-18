package event

import "github.com/gin-contrib/sse"

// swagger:response Stream
type _ struct {
	// in: body
	_ sse.Event
}
