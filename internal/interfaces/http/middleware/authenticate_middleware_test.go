package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	authcmd "go-api/internal/application/command/auth"
	identitycmd "go-api/internal/application/command/identity"
	usercmd "go-api/internal/application/command/user"
	"go-api/internal/domain/event"
	"go-api/internal/domain/port"
	domainuser "go-api/internal/domain/user"

	"github.com/gofiber/fiber/v3"
	"github.com/golang-jwt/jwt/v5"
)

const testSecret = "test-secret"

type fakeKeys struct{}

func (fakeKeys) GetKeyfunc() jwt.Keyfunc {
	return func(*jwt.Token) (any, error) { return []byte(testSecret), nil }
}
func (fakeKeys) GetIssuer() string { return "test-issuer" }

// fakeUsers implements only what the middleware path touches.
type fakeUsers struct {
	domainuser.UserWriteRepository
	byClerkID map[string]*domainuser.User
	lookupErr error
}

func (f *fakeUsers) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}
func (f *fakeUsers) GetByClerkID(_ context.Context, id string) (*domainuser.User, error) {
	if f.lookupErr != nil {
		return nil, f.lookupErr
	}
	return f.byClerkID[id], nil
}
func (f *fakeUsers) Save(_ context.Context, u *domainuser.User) error {
	f.byClerkID[u.ClerkID] = u
	return nil
}

type fakeOutbox struct{ port.OutboxRepository }

func (fakeOutbox) StoreEvents(context.Context, []event.DomainEvent) error { return nil }

type fakeClerk struct {
	user port.ClerkUser
	err  error
}

func (f fakeClerk) Get(context.Context, string) (port.ClerkUser, error) { return f.user, f.err }

func token(t *testing.T, clerkID string) string {
	t.Helper()
	claims := authcmd.JWTClaims{
		UserID: clerkID,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "test-issuer",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(testSecret))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func newApp(users *fakeUsers, clerk port.ClerkUserFetcher) *fiber.App {
	m := NewAuthenticateMiddleware(
		authcmd.NewValidateTokenHandler(fakeKeys{}, users),
		identitycmd.NewFetchUserHandler(clerk),
		usercmd.NewCreateUserHandler(users, fakeOutbox{}, port.SystemClock{}),
	)
	app := fiber.New()
	app.Get("/", m.Protected(), func(c fiber.Ctx) error { return c.SendStatus(http.StatusOK) })
	return app
}

func call(t *testing.T, app *fiber.App, authz string) int {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if authz != "" {
		req.Header.Set("Authorization", authz)
	}
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode
}

func TestAuthenticate_Rejections(t *testing.T) {
	existing := domainuser.NewUser("clerk_1", "A", "B", "a@x.test", false, testNow)
	banned := domainuser.NewUser("clerk_2", "C", "D", "c@x.test", true, testNow)
	users := &fakeUsers{byClerkID: map[string]*domainuser.User{"clerk_1": existing, "clerk_2": banned}}
	app := newApp(users, fakeClerk{})

	cases := map[string]struct {
		authz string
		want  int
	}{
		"missing header": {"", http.StatusUnauthorized},
		"wrong scheme":   {"Basic abc", http.StatusUnauthorized},
		"garbage token":  {"Bearer not-a-jwt", http.StatusUnauthorized},
		"banned user":    {"Bearer " + token(t, "clerk_2"), http.StatusUnauthorized},
		"valid existing": {"Bearer " + token(t, "clerk_1"), http.StatusOK},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := call(t, app, tc.authz); got != tc.want {
				t.Fatalf("status: got %d want %d", got, tc.want)
			}
		})
	}
}

func TestAuthenticate_InfrastructureFailuresAre503(t *testing.T) {
	t.Run("database lookup fails", func(t *testing.T) {
		users := &fakeUsers{byClerkID: map[string]*domainuser.User{}, lookupErr: errors.New("db down")}
		app := newApp(users, fakeClerk{})
		if got := call(t, app, "Bearer "+token(t, "clerk_9")); got != http.StatusServiceUnavailable {
			t.Fatalf("status: got %d want 503", got)
		}
	})

	t.Run("clerk fetch fails for a new user", func(t *testing.T) {
		users := &fakeUsers{byClerkID: map[string]*domainuser.User{}}
		app := newApp(users, fakeClerk{err: errors.New("clerk down")})
		if got := call(t, app, "Bearer "+token(t, "clerk_9")); got != http.StatusServiceUnavailable {
			t.Fatalf("status: got %d want 503", got)
		}
	})
}

func TestAuthenticate_FirstLoginProvisionsUser(t *testing.T) {
	users := &fakeUsers{byClerkID: map[string]*domainuser.User{}}
	clerk := fakeClerk{user: port.ClerkUser{ID: "clerk_9", FirstName: "N", Email: "n@x.test"}}
	app := newApp(users, clerk)

	if got := call(t, app, "Bearer "+token(t, "clerk_9")); got != http.StatusOK {
		t.Fatalf("status: got %d want 200", got)
	}
	if _, ok := users.byClerkID["clerk_9"]; !ok {
		t.Fatal("user must be provisioned on first login")
	}
}
