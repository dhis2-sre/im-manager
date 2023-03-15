package handler

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

func oneOf(fl validator.FieldLevel) bool {
	matches := strings.Split(fl.Param(), " ")
	value := fl.Field().String()
	for _, match := range matches {
		if match == value {
			return true
		}
	}
	return false
}

// RegisterValidation Inspiration: https://blog.logrocket.com/gin-binding-in-go-a-tutorial-with-examples/
func RegisterValidation() error {
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		return v.RegisterValidation("oneOf", oneOf)
	}
	return fmt.Errorf("error getting validation engine")
}
