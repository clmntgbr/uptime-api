package realtime

const (
	ActionCreated   = "created"
	ActionUpdated   = "updated"
	ActionDeleted   = "deleted"
	ActionCompleted = "completed"

	EntityUser = "user"
	EntityRun  = "run"
)

func EventType(entity, action string) string {
	return entity + "." + action
}
