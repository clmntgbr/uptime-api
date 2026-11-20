package context

import (
	"net/http/httptest"
	"testing"

	domainuser "go-api/internal/domain/user"

	"github.com/gofiber/fiber/v3"
)

func TestSetGetUser_RoundTrip(t *testing.T) {
	app := fiber.New()
	app.Get("/", func(c fiber.Ctx) error {
		if _, err := GetUser(c); err == nil {
			t.Error("GetUser without SetUser must fail")
		}
		SetUser(c, *domainuser.NewUser("clerk_1", "A", "B", "a@x.test", false))
		u, err := GetUser(c)
		if err != nil || u.ClerkID != "clerk_1" {
			t.Errorf("round trip: user=%v err=%v", u, err)
		}
		return c.SendStatus(fiber.StatusOK)
	})
	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("status %v err %v", resp, err)
	}
}
