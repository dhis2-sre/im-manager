package integration

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMapResponseToNames(t *testing.T) {
	results := dockerHubResponse{
		Results: []struct {
			Name string
		}{
			{
				Name: "2.39.0",
			},
			{
				Name: "2.38.0",
			},
			{
				Name: "2.37.0",
			},
		},
	}

	tags := mapResponseToNames(results)

	assert.ElementsMatch(t, tags, []string{"2.39.0", "2.38.0", "2.37.0"})
}
