package auth

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/danielgtaylor/huma/v2"
	sm "github.com/faciam-dev/gcfm/internal/server/middleware"
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
	if err != nil || u == nil {
		return nil, huma.Error401Unauthorized("invalid credentials")
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(in.Body.Password)) != nil {
		return nil, huma.Error401Unauthorized("invalid credentials")
	}
	tok, err := h.JWT.Generate(u.ID)
	if err != nil {
		return nil, err
	}
	return &loginOutput{Body: tokenResponse{AccessToken: tok, ExpiresAt: time.Now().Add(h.JWT.exp)}}, nil
}

type refreshInput struct{}

func (h *Handler) refresh(ctx context.Context, _ *refreshInput) (*loginOutput, error) {
	sub := sm.UserFromContext(ctx)
	uid, err := strconv.ParseUint(sub, 10, 64)
	if err != nil || sub == "" {
		return nil, huma.Error401Unauthorized("unauthorized")
	}
	tok, err := h.JWT.Generate(uid)
	if err != nil {
		return nil, err
	}
	return &loginOutput{Body: tokenResponse{AccessToken: tok, ExpiresAt: time.Now().Add(h.JWT.exp)}}, nil
}
