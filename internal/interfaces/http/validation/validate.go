package validation

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"
)

// Rejection is a client error. Handlers return it instead of writing the
// response themselves; FiberErrorHandler writes Status and Body exactly once.
type Rejection struct {
	Status int
	Body   fiber.Map
}

func (r *Rejection) Error() string {
	if msg, ok := r.Body["message"].(string); ok {
		return msg
	}
	return "request rejected"
}

// Reject builds a client-error response for the error handler to write.
func Reject(status int, body fiber.Map) error {
	return &Rejection{Status: status, Body: body}
}

var validate *validator.Validate

func init() {
	validate = validator.New()
	validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		if name := tagName(fld, "json"); name != "" {
			return name
		}
		if name := tagName(fld, "query"); name != "" {
			return name
		}
		return fld.Name
	})
}

func tagName(fld reflect.StructField, key string) string {
	name := strings.SplitN(fld.Tag.Get(key), ",", 2)[0]
	if name == "" || name == "-" {
		return ""
	}
	return name
}

func BindBody(c fiber.Ctx, dst any) error {
	if err := c.Bind().Body(dst); err != nil {
		return Reject(fiber.StatusBadRequest, fiber.Map{"message": "Invalid request body"})
	}
	return Struct(c, dst)
}

func BindAndValidateQuery(c fiber.Ctx, dst any) error {
	if err := c.Bind().Query(dst); err != nil {
		return Reject(fiber.StatusBadRequest, fiber.Map{"message": "Invalid query parameters"})
	}
	return Struct(c, dst)
}

func Struct(_ fiber.Ctx, dst any) error {
	trimRequiredStrings(dst)
	if err := validate.Struct(dst); err != nil {
		return Reject(fiber.StatusBadRequest, fiber.Map{
			"message": "Validation failed",
			"errors":  formatErrors(err),
		})
	}
	return nil
}

// FiberErrorHandler writes a Rejection as the client error it describes.
// Other errors use the standard envelope `{ message }` (never `{ success, data }`).
func FiberErrorHandler(c fiber.Ctx, err error) error {
	var rejection *Rejection
	if errors.As(err, &rejection) {
		return c.Status(rejection.Status).JSON(rejection.Body)
	}

	code := fiber.StatusInternalServerError
	message := "Internal server error"
	var e *fiber.Error
	if errors.As(err, &e) {
		code = e.Code
		if code >= 400 && code < 500 && e.Message != "" {
			message = e.Message
		}
	}

	return c.Status(code).JSON(fiber.Map{
		"message": message,
	})
}

func formatErrors(err error) map[string]string {
	out := make(map[string]string)
	validationErrors, ok := err.(validator.ValidationErrors)
	if !ok {
		out["_error"] = err.Error()
		return out
	}

	for _, fieldErr := range validationErrors {
		out[fieldErr.Field()] = messageFor(fieldErr)
	}
	return out
}

func messageFor(fieldErr validator.FieldError) string {
	switch fieldErr.Tag() {
	case "required":
		return "is required"
	case "uuid":
		return "must be a valid UUID"
	case "email":
		return "must be a valid email"
	case "url":
		return "must be a valid URL"
	case "min":
		if fieldErr.Kind() == reflect.String {
			return fmt.Sprintf("must be at least %s characters", fieldErr.Param())
		}
		return fmt.Sprintf("must be at least %s", fieldErr.Param())
	case "max":
		if fieldErr.Kind() == reflect.String {
			return fmt.Sprintf("must be at most %s characters", fieldErr.Param())
		}
		return fmt.Sprintf("must be at most %s", fieldErr.Param())
	case "gte":
		return fmt.Sprintf("must be greater than or equal to %s", fieldErr.Param())
	case "lte":
		return fmt.Sprintf("must be less than or equal to %s", fieldErr.Param())
	case "oneof":
		return "is invalid"
	case "dive":
		return "contains invalid items"
	default:
		return fmt.Sprintf("failed on '%s' validation", fieldErr.Tag())
	}
}

// trimRequiredStrings trims top-level string fields tagged `required`, so a name
// of only spaces fails `required` and stored values carry no stray whitespace.
func trimRequiredStrings(dst any) {
	v := reflect.ValueOf(dst)
	if v.Kind() != reflect.Pointer || v.IsNil() {
		return
	}
	v = v.Elem()
	if v.Kind() != reflect.Struct {
		return
	}
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.Type.Kind() != reflect.String || !strings.Contains(field.Tag.Get("validate"), "required") {
			continue
		}
		if fv := v.Field(i); fv.CanSet() {
			fv.SetString(strings.TrimSpace(fv.String()))
		}
	}
}
