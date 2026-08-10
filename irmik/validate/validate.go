// Package validate provides request/struct validation helpers for Gin,
// built on go-playground/validator.
package validate

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

var (
	once     sync.Once
	validate *validator.Validate
)

func engine() *validator.Validate {
	once.Do(func() {
		validate = validator.New(validator.WithRequiredStructEnabled())
	})
	return validate
}

// Errors maps field names to validation messages.
type Errors map[string][]string

func (e Errors) Error() string {
	if len(e) == 0 {
		return "validation failed"
	}
	parts := make([]string, 0, len(e))
	for field, msgs := range e {
		parts = append(parts, field+": "+strings.Join(msgs, "; "))
	}
	return "validation failed: " + strings.Join(parts, ", ")
}

// Empty reports whether there are no field errors.
func (e Errors) Empty() bool { return len(e) == 0 }

// Add appends a message for field.
func (e Errors) Add(field, message string) {
	e[field] = append(e[field], message)
}

// AsErrors extracts Errors from err, if present.
func AsErrors(err error) (Errors, bool) {
	var ve Errors
	if errors.As(err, &ve) {
		return ve, true
	}
	return nil, false
}

// Struct validates v using struct tags (e.g. `validate:"required,email"`).
func Struct(v any) error {
	err := engine().Struct(v)
	if err == nil {
		return nil
	}
	return fromValidator(err)
}

// Var validates a single value with the given tag.
func Var(field any, tag string) error {
	err := engine().Var(field, tag)
	if err == nil {
		return nil
	}
	return fromValidator(err)
}

// BindJSON binds JSON into dst then validates it.
func BindJSON(c *gin.Context, dst any) error {
	if err := c.ShouldBindBodyWith(dst, binding.JSON); err != nil {
		return wrapBind(err)
	}
	return Struct(dst)
}

// BindForm binds form data into dst then validates it.
func BindForm(c *gin.Context, dst any) error {
	if err := c.ShouldBind(dst); err != nil {
		return wrapBind(err)
	}
	return Struct(dst)
}

// BindQuery binds query parameters into dst then validates it.
func BindQuery(c *gin.Context, dst any) error {
	if err := c.ShouldBindQuery(dst); err != nil {
		return wrapBind(err)
	}
	return Struct(dst)
}

// Abort writes a 400 JSON response for validation/bind errors and aborts.
func Abort(c *gin.Context, err error) {
	if ve, ok := AsErrors(err); ok {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error":  "validation_failed",
			"fields": ve,
		})
		return
	}
	c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
		"error":   "bad_request",
		"message": err.Error(),
	})
}

func fromValidator(err error) error {
	var verrs validator.ValidationErrors
	if !errors.As(err, &verrs) {
		return err
	}
	out := make(Errors, len(verrs))
	for _, fe := range verrs {
		field := fe.Field()
		out.Add(field, messageFor(fe))
	}
	return out
}

func wrapBind(err error) error {
	if err == nil {
		return nil
	}
	var verrs validator.ValidationErrors
	if errors.As(err, &verrs) {
		return fromValidator(err)
	}
	var syn *json.SyntaxError
	if errors.As(err, &syn) {
		e := make(Errors)
		e.Add("_", "invalid JSON")
		return e
	}
	e := make(Errors)
	e.Add("_", err.Error())
	return e
}

func messageFor(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "is required"
	case "email":
		return "must be a valid email"
	case "min":
		return fmt.Sprintf("must be at least %s", fe.Param())
	case "max":
		return fmt.Sprintf("must be at most %s", fe.Param())
	case "len":
		return fmt.Sprintf("must be length %s", fe.Param())
	case "oneof":
		return fmt.Sprintf("must be one of [%s]", fe.Param())
	case "eqfield":
		return fmt.Sprintf("must equal %s", fe.Param())
	default:
		if fe.Param() != "" {
			return fmt.Sprintf("failed on '%s' (%s)", fe.Tag(), fe.Param())
		}
		return fmt.Sprintf("failed on '%s'", fe.Tag())
	}
}
