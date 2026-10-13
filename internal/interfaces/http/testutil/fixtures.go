package testutil

import "github.com/google/uuid"

// Stable UUIDs shared across handler tests.
var (
	TestUserID    = uuid.MustParse("01960000-0000-7000-8000-000000000001")
	TestProjectID = uuid.MustParse("01960000-0000-7000-8000-000000000002")
)
