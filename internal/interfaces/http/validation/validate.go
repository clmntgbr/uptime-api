package validation

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"
)

var ErrValidationFailed = errors.New("validation failed")

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
		_ = c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid request body",
		})
		return ErrValidationFailed
	}
	return Struct(c, dst)
}

func BindAndValidateQuery(c fiber.Ctx, dst any) error {
	if err := c.Bind().Query(dst); err != nil {
		_ = c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid query parameters",
		})
		return ErrValidationFailed
	}
	return Struct(c, dst)
}

func Struct(c fiber.Ctx, dst any) error {
	if err := validate.Struct(dst); err != nil {
		_ = c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Validation failed",
			"errors":  formatErrors(err),
		})
		return ErrValidationFailed
	}
	return nil
}

// FiberErrorHandler suppresses ErrValidationFailed because the response is already written.
func FiberErrorHandler(c fiber.Ctx, err error) error {
	if errors.Is(err, ErrValidationFailed) {
		return nil
	}
	return fiber.DefaultErrorHandler(c, err)
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
