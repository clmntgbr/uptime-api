package main

import (
	"go-api/cmd/api/di"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/healthcheck"
)

func setupRoutes(app *fiber.App, container *di.Container) {
	setupHealthChecks(app)
	setupWebhooks(app, container)
	setupAPIRoutes(app, container)
}

func setupWebhooks(app *fiber.App, container *di.Container) {
	webhooks := app.Group("/webhooks")
	webhooks.Post("/clerk", container.UserWebhookMiddleware.Protected(), container.UserWebhookHandler.Execute)
}

func setupHealthChecks(app *fiber.App) {
	app.Get(healthcheck.LivenessEndpoint, healthcheck.New())
	app.Get(healthcheck.ReadinessEndpoint, healthcheck.New())
	app.Get(healthcheck.StartupEndpoint, healthcheck.New())
}

func setupAPIRoutes(app *fiber.App, container *di.Container) {
	public := app.Group("/api")

	protected := public.Group("", container.AuthenticateMiddleware.Protected())
	setupUserRoutes(protected, container)
	setupRealtimeRoutes(protected, container)
	setupVariableRoutes(protected, container)
	setupWorkflowRoutes(protected, container)
}

func setupVariableRoutes(api fiber.Router, container *di.Container) {
	api.Get("/variables", container.VariableHandler.ListGlobals)
	api.Post("/variables", container.VariableHandler.CreateGlobal)
	api.Get("/variables/:id", container.VariableHandler.GetGlobal)
	api.Put("/variables/:id", container.VariableHandler.UpdateGlobal)
	api.Delete("/variables/:id", container.VariableHandler.DeleteGlobal)
}

func setupWorkflowRoutes(api fiber.Router, container *di.Container) {
	api.Get("/workflows", container.WorkflowHandler.List)
	api.Post("/workflows", container.WorkflowHandler.Create)
	api.Get("/workflows/:id", container.WorkflowHandler.GetByID)
	api.Put("/workflows/:id", container.WorkflowHandler.Update)
	api.Delete("/workflows/:id", container.WorkflowHandler.Delete)
	api.Post("/workflows/:id/runs", container.WorkflowHandler.Run)

	api.Get("/workflows/:id/variables", container.VariableHandler.ListStatics)
	api.Post("/workflows/:id/variables", container.VariableHandler.CreateStatic)
	api.Get("/workflows/:id/variables/:name", container.VariableHandler.GetStatic)
	api.Put("/workflows/:id/variables/:name", container.VariableHandler.UpdateStatic)
	api.Delete("/workflows/:id/variables/:name", container.VariableHandler.DeleteStatic)

	api.Get("/workflows/:id/calls", container.CallHandler.List)
	api.Post("/workflows/:id/calls", container.CallHandler.Create)
	api.Get("/workflows/:id/calls/:callId", container.CallHandler.GetByID)
	api.Put("/workflows/:id/calls/:callId", container.CallHandler.Update)
	api.Delete("/workflows/:id/calls/:callId", container.CallHandler.Delete)

	api.Get("/workflows/:id/calls/:callId/available-variables", container.VariableHandler.ListAvailable)
	api.Get("/workflows/:id/calls/:callId/variables", container.VariableHandler.ListCallVariables)
	api.Post("/workflows/:id/calls/:callId/variables", container.VariableHandler.CreateCallVariable)
	api.Get("/workflows/:id/calls/:callId/variables/:name", container.VariableHandler.GetCallVariable)
	api.Put("/workflows/:id/calls/:callId/variables/:name", container.VariableHandler.UpdateCallVariable)
	api.Delete("/workflows/:id/calls/:callId/variables/:name", container.VariableHandler.DeleteCallVariable)
}

func setupRealtimeRoutes(api fiber.Router, container *di.Container) {
	api.Get("/realtime/connection", container.RealtimeHandler.GetConnection)
}

func setupUserRoutes(api fiber.Router, container *di.Container) {
	api.Get("/users/me", container.UserHandler.GetUser)
}
