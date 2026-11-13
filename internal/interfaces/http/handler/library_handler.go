package handler

import (
	cmdlibrary "go-api/internal/application/command/library"
	querylibrary "go-api/internal/application/query/library"
	domainlibrary "go-api/internal/domain/library"
	"go-api/internal/domain/paginate"
	httpctx "go-api/internal/interfaces/http/context"
	"go-api/internal/interfaces/http/dto"
	"go-api/internal/interfaces/http/presenter"
	"go-api/internal/interfaces/http/validation"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type LibraryHandler struct {
	createHandler        libraryCreateHandler
	updateHandler        libraryUpdateHandler
	deleteHandler        libraryDeleteHandler
	importHandler        libraryImportHandler
	listHandler          libraryListHandler
	getHandler           libraryGetHandler
	previewImportHandler libraryPreviewImportHandler
	saveFromCallHandler  librarySaveFromCallHandler
}

func NewLibraryHandler(
	createHandler libraryCreateHandler,
	updateHandler libraryUpdateHandler,
	deleteHandler libraryDeleteHandler,
	importHandler libraryImportHandler,
	listHandler libraryListHandler,
	getHandler libraryGetHandler,
	previewImportHandler libraryPreviewImportHandler,
	saveFromCallHandler librarySaveFromCallHandler,
) *LibraryHandler {
	return &LibraryHandler{
		createHandler:        createHandler,
		updateHandler:        updateHandler,
		deleteHandler:        deleteHandler,
		importHandler:        importHandler,
		listHandler:          listHandler,
		getHandler:           getHandler,
		previewImportHandler: previewImportHandler,
		saveFromCallHandler:  saveFromCallHandler,
	}
}

func (h *LibraryHandler) List(c fiber.Ctx) error {
	user, err := httpctx.GetUser(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "Unauthorized"})
	}

	var listQuery paginate.PaginateQuery
	if err := validation.BindAndValidateQuery(c, &listQuery); err != nil {
		return err
	}
	sortBy, orderBy := listQuery.SortBy, listQuery.OrderBy
	listQuery.Normalize()
	if sortBy == "" {
		listQuery.SortBy = "createdAt"
	}
	if orderBy == "" {
		listQuery.OrderBy = paginate.OrderByDesc
	}

	views, total, err := h.listHandler.Handle(c.Context(), querylibrary.ListItemsQuery{
		UserID: user.ID,
		Query:  listQuery,
	})
	if err != nil {
		return writeLibraryError(c, err, "Failed to list library items")
	}
	return c.Status(fiber.StatusOK).JSON(paginate.NewPaginateResponse(
		presenter.NewLibraryListItemResponses(views),
		int(total),
		listQuery,
	))
}

func (h *LibraryHandler) GetByID(c fiber.Ctx) error {
	user, err := httpctx.GetUser(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "Unauthorized"})
	}
	itemID, err := parseLibraryItemID(c)
	if err != nil {
		return err
	}

	item, err := h.getHandler.Handle(c.Context(), querylibrary.GetItemQuery{UserID: user.ID, ItemID: itemID})
	if err != nil {
		return writeLibraryError(c, err, "Failed to get library item")
	}
	return c.Status(fiber.StatusOK).JSON(presenter.NewLibraryItemResponse(*item))
}

func (h *LibraryHandler) Create(c fiber.Ctx) error {
	user, err := httpctx.GetUser(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "Unauthorized"})
	}
	def, err := bindCallDefinition(c)
	if err != nil {
		return err
	}

	item, err := h.createHandler.Handle(c.Context(), cmdlibrary.CreateItemCommand{
		UserID:     user.ID,
		Definition: def,
	})
	if err != nil {
		return writeLibraryError(c, err, "Failed to create library item")
	}
	return c.Status(fiber.StatusCreated).JSON(presenter.NewLibraryItemResponse(*item))
}

func (h *LibraryHandler) Update(c fiber.Ctx) error {
	user, err := httpctx.GetUser(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "Unauthorized"})
	}
	itemID, err := parseLibraryItemID(c)
	if err != nil {
		return err
	}
	def, err := bindCallDefinition(c)
	if err != nil {
		return err
	}

	item, err := h.updateHandler.Handle(c.Context(), cmdlibrary.UpdateItemCommand{
		UserID:     user.ID,
		ItemID:     itemID,
		Definition: def,
	})
	if err != nil {
		return writeLibraryError(c, err, "Failed to update library item")
	}
	return c.Status(fiber.StatusOK).JSON(presenter.NewLibraryItemResponse(*item))
}

func (h *LibraryHandler) Delete(c fiber.Ctx) error {
	user, err := httpctx.GetUser(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "Unauthorized"})
	}
	itemID, err := parseLibraryItemID(c)
	if err != nil {
		return err
	}

	if err := h.deleteHandler.Handle(c.Context(), cmdlibrary.DeleteItemCommand{
		UserID: user.ID,
		ItemID: itemID,
	}); err != nil {
		return writeLibraryError(c, err, "Failed to delete library item")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// PreviewImport parses the content and returns candidates. Nothing is stored.
func (h *LibraryHandler) PreviewImport(c fiber.Ctx) error {
	var body dto.LibraryImportPreviewRequest
	if err := validation.BindBody(c, &body); err != nil {
		return err
	}

	result, err := h.previewImportHandler.Handle(c.Context(), querylibrary.PreviewImportQuery{
		Format:  domainlibrary.Format(body.Format),
		Content: []byte(body.Content),
		BaseURL: body.BaseURL,
	})
	if err != nil {
		return writeLibraryError(c, err, "Failed to preview import")
	}
	return c.Status(fiber.StatusOK).JSON(presenter.NewLibraryImportPreviewResponse(result.Candidates, result.Skipped))
}

// Import stores the candidates confirmed by the user, all or nothing.
func (h *LibraryHandler) Import(c fiber.Ctx) error {
	user, err := httpctx.GetUser(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "Unauthorized"})
	}
	items, err := bindLibraryImport(c)
	if err != nil {
		return err
	}

	created, err := h.importHandler.Handle(c.Context(), cmdlibrary.ImportCommand{
		UserID: user.ID,
		Items:  items,
	})
	if err != nil {
		return writeLibraryError(c, err, "Failed to import library items")
	}
	return c.Status(fiber.StatusCreated).JSON(presenter.NewLibraryItemListResponse(created))
}

// SaveFromCall stores the definition of a call the user owns, whatever its workflow.
func (h *LibraryHandler) SaveFromCall(c fiber.Ctx) error {
	user, err := httpctx.GetUser(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "Unauthorized"})
	}
	callID, err := uuid.Parse(c.Params("callId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Invalid call id"})
	}

	item, err := h.saveFromCallHandler.Handle(c.Context(), cmdlibrary.SaveFromCallCommand{
		UserID: user.ID,
		CallID: callID,
	})
	if err != nil {
		return writeLibraryError(c, err, "Failed to save call to library")
	}
	return c.Status(fiber.StatusCreated).JSON(presenter.NewLibraryItemResponse(*item))
}
