package realtime

const (
	ActionCreated = "created"
	ActionUpdated = "updated"
	ActionDeleted = "deleted"

	EntityUser     = "user"
	EntityWorkflow = "workflow"
)

func EventType(entity, action string) string {
	return entity + "." + action
}
