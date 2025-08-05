package auth

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/faciam-dev/gcfm/internal/logger"
	sm "github.com/faciam-dev/gcfm/internal/server/middleware"
	"github.com/faciam-dev/gcfm/internal/tenant"
	"golang.org/x/crypto/bcrypt"
)

type Handler struct {
	Repo *UserRepo
	JWT  *JWT
}

type loginBody struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type tokenResponse struct {
	AccessToken string    `json:"access_token"`
	ExpiresAt   time.Time `json:"expires_at"`
}

type loginInput struct {
	Body loginBody
}

type loginOutput struct {
	Body tokenResponse
}

func Register(api huma.API, h *Handler) {
	huma.Register(api, huma.Operation{
		OperationID: "login",
		Method:      http.MethodPost,
		Path:        "/v1/auth/login",
		Summary:     "Login",
		Tags:        []string{"Auth"},
	}, h.login)

	huma.Register(api, huma.Operation{
		OperationID: "refresh",
		Method:      http.MethodPost,
		Path:        "/v1/auth/refresh",
		Summary:     "Refresh token",
		Tags:        []string{"Auth"},
	}, h.refresh)
}

func (h *Handler) login(ctx context.Context, in *loginInput) (*loginOutput, error) {
	u, err := h.Repo.GetByUsername(ctx, in.Body.Username)
	if err != nil {
		logger.L.Error("get user", "err", err)
		if isDatabaseError(err) {
			return nil, huma.Error500InternalServerError("internal server error")
		}
		return nil, huma.Error401Unauthorized("invalid credentials")
	}
	if u == nil {
		logger.L.Info("user not found", "username", in.Body.Username)
		return nil, huma.Error401Unauthorized("invalid credentials")
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(in.Body.Password)) != nil {
		logger.L.Info("password mismatch", "username", in.Body.Username)
		return nil, huma.Error401Unauthorized("invalid credentials")
	}
	tenantID := tenant.FromContext(ctx)
	roles, err := h.Repo.GetRoles(ctx, u.ID)
	if err != nil {
		logger.L.Error("get roles", "err", err)
		if isDatabaseError(err) {
			return nil, huma.Error500InternalServerError("internal server error")
		}
		return nil, err
	}
	tok, err := h.JWT.GenerateWithTenant(u.ID, tenantID, roles)
	if err != nil {
		logger.L.Error("generate token", "err", err)
		return nil, err
	}
	logger.L.Info("user logged in", "userID", u.ID, "tenant", tenantID)
	return &loginOutput{Body: tokenResponse{AccessToken: tok, ExpiresAt: time.Now().Add(h.JWT.exp)}}, nil
}

type refreshInput struct{}

func (h *Handler) refresh(ctx context.Context, _ *refreshInput) (*loginOutput, error) {
	sub := sm.UserFromContext(ctx)
	uid, err := strconv.ParseUint(sub, 10, 64)
	if err != nil || sub == "" {
		return nil, huma.Error401Unauthorized("unauthorized")
	}
	tenantID := tenant.FromContext(ctx)
	roles, err := h.Repo.GetRoles(ctx, uid)
	if err != nil {
		logger.L.Error("get roles", "err", err)
		if isDatabaseError(err) {
			return nil, huma.Error500InternalServerError("internal server error")
		}
		return nil, err
	}
	tok, err := h.JWT.GenerateWithTenant(uid, tenantID, roles)
	if err != nil {
		return nil, err
	}
	return &loginOutput{Body: tokenResponse{AccessToken: tok, ExpiresAt: time.Now().Add(h.JWT.exp)}}, nil
}

// isDatabaseError determines if the provided error likely came from the
// database layer. Currently any non-nil error is treated as a DB error since
// GetByUsername hides sql.ErrNoRows.
func isDatabaseError(err error) bool {
	if err == nil {
		return false
	}
	return !errors.Is(err, sql.ErrNoRows)
}
