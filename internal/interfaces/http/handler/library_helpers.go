package handler

import (
	"errors"
	"fmt"

	cmdlibrary "go-api/internal/application/command/library"
	domaincall "go-api/internal/domain/call"
	domainlibrary "go-api/internal/domain/library"
	domainworkflow "go-api/internal/domain/workflow"
	"go-api/internal/interfaces/http/dto"
	"go-api/internal/interfaces/http/validation"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

func parseLibraryItemID(c fiber.Ctx) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		_ = c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Invalid library item id"})
		return uuid.Nil, validation.ErrValidationFailed
	}
	return id, nil
}

// bindLibraryImport validates the confirmation body and maps each candidate to a
// definition. Rule errors are keyed by items[i].payload.rules[j].
func bindLibraryImport(c fiber.Ctx) ([]cmdlibrary.ImportItem, error) {
	var body dto.LibraryImportRequest
	if err := validation.BindBody(c, &body); err != nil {
		return nil, err
	}

	items := make([]cmdlibrary.ImportItem, 0, len(body.Items))
	errs := map[string]string{}
	for i, in := range body.Items {
		def, ruleErrors := callDefinitionFromRequest(in.Payload)
		for key, msg := range ruleErrors {
			errs[fmt.Sprintf("items[%d].payload.%s", i, key)] = msg
		}
		if in.Name != "" {
			def.Name = in.Name
		}
		items = append(items, cmdlibrary.ImportItem{Key: in.Key, Definition: def})
	}
	if len(errs) > 0 {
		_ = c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Validation failed",
			"errors":  errs,
		})
		return nil, validation.ErrValidationFailed
	}
	return items, nil
}

func writeLibraryError(c fiber.Ctx, err error, fallback string) error {
	var importErr *domainlibrary.ImportError
	var verr *domaincall.ValidationError
	switch {
	case errors.Is(err, domainlibrary.ErrNotFound):
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"message": "Library item not found"})
	case errors.Is(err, domainworkflow.ErrCallNotFound):
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"message": "Call not found"})
	case errors.Is(err, domainlibrary.ErrImportTooLarge):
		return c.Status(fiber.StatusRequestEntityTooLarge).JSON(fiber.Map{"message": "Import content exceeds 2 MB"})
	case errors.Is(err, domainlibrary.ErrInvalidBaseURL):
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": domainlibrary.ErrInvalidBaseURL.Error()})
	case errors.Is(err, domainlibrary.ErrUnsupportedFormat):
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Unsupported import format"})
	case errors.As(err, &importErr):
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"message": importErr.Message})
	case errors.As(err, &verr):
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": verr.Error(),
			"code":    "INVALID_CALL",
		})
	default:
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": fallback})
	}
}
