package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"code-kanban/api/h"
	"code-kanban/utils"
)

type authStatusItem struct {
	Enabled              bool   `json:"enabled"`
	Authenticated        bool   `json:"authenticated"`
	FrontendSalt         string `json:"frontendSalt"`
	FrontendPBKDF2Rounds int    `json:"frontendPBKDF2Rounds"`
	SessionTTLSeconds    int64  `json:"sessionTtlSeconds"`
}

type authClientHashInput struct {
	ClientHash string `json:"clientHash"`
}

type authChangePasswordInput struct {
	CurrentClientHash string `json:"currentClientHash"`
	NewClientHash     string `json:"newClientHash"`
}

func registerAuthMiddleware(app *fiber.App, cfg *utils.AppConfig) {
	app.Use(func(ctx *fiber.Ctx) error {
		if !utils.AuthEnabled(cfg) {
			return ctx.Next()
		}

		path := ctx.Path()
		if isAnonymousPath(path, cfg) {
			return ctx.Next()
		}
		if !requiresAuth(path, cfg) {
			return ctx.Next()
		}

		if ok, _ := isRequestAuthenticated(ctx, cfg); ok {
			return ctx.Next()
		}

		clearAuthCookie(ctx)
		return sendAPIError(ctx, http.StatusUnauthorized, "authentication required")
	})
}

func registerAuthRoutes(app *fiber.App, cfg *utils.AppConfig) {
	app.Get("/api/v1/auth/status", func(ctx *fiber.Ctx) error {
		authenticated, err := isRequestAuthenticated(ctx, cfg)
		if err != nil {
			clearAuthCookie(ctx)
		}

		resp := h.NewItemResponse(authStatusItem{
			Enabled:              utils.AuthEnabled(cfg),
			Authenticated:        authenticated,
			FrontendSalt:         cfg.Auth.FrontendSalt,
			FrontendPBKDF2Rounds: utils.FrontendPBKDF2Iterations,
			SessionTTLSeconds:    int64(cfg.Auth.SessionDuration().Seconds()),
		})
		resp.Status = http.StatusOK
		return ctx.Status(http.StatusOK).JSON(resp)
	})

	app.Post("/api/v1/auth/login", func(ctx *fiber.Ctx) error {
		if !utils.AuthEnabled(cfg) {
			return sendAPIError(ctx, http.StatusConflict, "password protection is disabled")
		}

		var input authClientHashInput
		if err := ctx.BodyParser(&input); err != nil {
			return sendAPIError(ctx, http.StatusBadRequest, "invalid request body")
		}

		ok, err := utils.VerifyClientSecret(strings.TrimSpace(input.ClientHash), cfg.Auth.PasswordHash)
		if err != nil || !ok {
			return sendAPIError(ctx, http.StatusUnauthorized, "invalid password")
		}

		if err := issueAuthCookie(ctx, cfg); err != nil {
			return sendAPIError(ctx, http.StatusInternalServerError, "failed to issue session")
		}

		resp := h.NewMessageResponse("login successful")
		resp.Status = http.StatusOK
		return ctx.Status(http.StatusOK).JSON(resp)
	})

	app.Post("/api/v1/auth/logout", func(ctx *fiber.Ctx) error {
		clearAuthCookie(ctx)
		resp := h.NewMessageResponse("logout successful")
		resp.Status = http.StatusOK
		return ctx.Status(http.StatusOK).JSON(resp)
	})

	app.Post("/api/v1/auth/password/enable", func(ctx *fiber.Ctx) error {
		if utils.AuthEnabled(cfg) {
			return sendAPIError(ctx, http.StatusConflict, "password protection is already enabled")
		}

		var input authClientHashInput
		if err := ctx.BodyParser(&input); err != nil {
			return sendAPIError(ctx, http.StatusBadRequest, "invalid request body")
		}

		passwordHash, err := utils.HashClientSecret(strings.TrimSpace(input.ClientHash))
		if err != nil {
			return sendAPIError(ctx, http.StatusBadRequest, "invalid password hash")
		}
		tokenSecret, err := utils.NewAuthTokenSecret()
		if err != nil {
			return sendAPIError(ctx, http.StatusInternalServerError, "failed to generate token secret")
		}

		if err := utils.UpdateConfig(cfg, func(c *utils.AppConfig) {
			c.Auth.PasswordHash = passwordHash
			c.Auth.TokenSecret = tokenSecret
		}); err != nil {
			return sendAPIError(ctx, http.StatusInternalServerError, "failed to save password configuration")
		}

		if err := issueAuthCookie(ctx, cfg); err != nil {
			return sendAPIError(ctx, http.StatusInternalServerError, "failed to issue session")
		}

		resp := h.NewMessageResponse("password protection enabled")
		resp.Status = http.StatusOK
		return ctx.Status(http.StatusOK).JSON(resp)
	})

	app.Post("/api/v1/auth/password/change", func(ctx *fiber.Ctx) error {
		if !utils.AuthEnabled(cfg) {
			return sendAPIError(ctx, http.StatusConflict, "password protection is disabled")
		}
		if ok, _ := isRequestAuthenticated(ctx, cfg); !ok {
			clearAuthCookie(ctx)
			return sendAPIError(ctx, http.StatusUnauthorized, "authentication required")
		}

		var input authChangePasswordInput
		if err := ctx.BodyParser(&input); err != nil {
			return sendAPIError(ctx, http.StatusBadRequest, "invalid request body")
		}

		currentOK, err := utils.VerifyClientSecret(strings.TrimSpace(input.CurrentClientHash), cfg.Auth.PasswordHash)
		if err != nil || !currentOK {
			return sendAPIError(ctx, http.StatusUnauthorized, "current password is invalid")
		}

		passwordHash, err := utils.HashClientSecret(strings.TrimSpace(input.NewClientHash))
		if err != nil {
			return sendAPIError(ctx, http.StatusBadRequest, "invalid new password hash")
		}
		tokenSecret, err := utils.NewAuthTokenSecret()
		if err != nil {
			return sendAPIError(ctx, http.StatusInternalServerError, "failed to generate token secret")
		}

		if err := utils.UpdateConfig(cfg, func(c *utils.AppConfig) {
			c.Auth.PasswordHash = passwordHash
			c.Auth.TokenSecret = tokenSecret
		}); err != nil {
			return sendAPIError(ctx, http.StatusInternalServerError, "failed to save password configuration")
		}

		if err := issueAuthCookie(ctx, cfg); err != nil {
			return sendAPIError(ctx, http.StatusInternalServerError, "failed to issue session")
		}

		resp := h.NewMessageResponse("password changed")
		resp.Status = http.StatusOK
		return ctx.Status(http.StatusOK).JSON(resp)
	})

	app.Post("/api/v1/auth/password/disable", func(ctx *fiber.Ctx) error {
		if !utils.AuthEnabled(cfg) {
			return sendAPIError(ctx, http.StatusConflict, "password protection is disabled")
		}
		if ok, _ := isRequestAuthenticated(ctx, cfg); !ok {
			clearAuthCookie(ctx)
			return sendAPIError(ctx, http.StatusUnauthorized, "authentication required")
		}

		var input authClientHashInput
		if err := ctx.BodyParser(&input); err != nil {
			return sendAPIError(ctx, http.StatusBadRequest, "invalid request body")
		}

		currentOK, err := utils.VerifyClientSecret(strings.TrimSpace(input.ClientHash), cfg.Auth.PasswordHash)
		if err != nil || !currentOK {
			return sendAPIError(ctx, http.StatusUnauthorized, "current password is invalid")
		}

		tokenSecret, err := utils.NewAuthTokenSecret()
		if err != nil {
			return sendAPIError(ctx, http.StatusInternalServerError, "failed to generate token secret")
		}

		if err := utils.UpdateConfig(cfg, func(c *utils.AppConfig) {
			c.Auth.PasswordHash = ""
			c.Auth.TokenSecret = tokenSecret
		}); err != nil {
			return sendAPIError(ctx, http.StatusInternalServerError, "failed to save password configuration")
		}

		clearAuthCookie(ctx)
		resp := h.NewMessageResponse("password protection disabled")
		resp.Status = http.StatusOK
		return ctx.Status(http.StatusOK).JSON(resp)
	})
}

func requiresAuth(path string, cfg *utils.AppConfig) bool {
	if strings.HasPrefix(path, "/api/v1/") {
		return true
	}
	if path == "/capture-debug" || path == "/openapi.json" {
		return true
	}

	docsPath := strings.TrimSpace(cfg.DocsPath)
	if docsPath == "" {
		return false
	}
	if !strings.HasPrefix(docsPath, "/") {
		docsPath = "/" + docsPath
	}
	return path == docsPath || strings.HasPrefix(path, docsPath+"/")
}

func isAnonymousPath(path string, cfg *utils.AppConfig) bool {
	if path == "/api/v1/health" {
		return true
	}
	if strings.HasPrefix(path, "/api/v1/auth/") {
		return true
	}
	return false
}

func isRequestAuthenticated(ctx *fiber.Ctx, cfg *utils.AppConfig) (bool, error) {
	token := readRequestToken(ctx)
	if token == "" {
		return false, nil
	}
	_, err := utils.VerifyAuthSessionToken(token, cfg.Auth.TokenSecret, time.Now())
	if err != nil {
		return false, err
	}
	return true, nil
}

func readRequestToken(ctx *fiber.Ctx) string {
	if ctx == nil {
		return ""
	}
	if token := strings.TrimSpace(ctx.Cookies(utils.AuthCookieName)); token != "" {
		return token
	}

	header := strings.TrimSpace(ctx.Get(fiber.HeaderAuthorization))
	if header == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(header), "bearer ") {
		return strings.TrimSpace(header[7:])
	}
	return header
}

func issueAuthCookie(ctx *fiber.Ctx, cfg *utils.AppConfig) error {
	token, err := utils.IssueAuthSessionToken(cfg.Auth.TokenSecret, cfg.Auth.SessionDuration(), time.Now())
	if err != nil {
		return err
	}

	ctx.Cookie(&fiber.Cookie{
		Name:     utils.AuthCookieName,
		Value:    token,
		Path:     "/",
		HTTPOnly: true,
		Secure:   strings.EqualFold(ctx.Protocol(), "https"),
		SameSite: fiber.CookieSameSiteLaxMode,
		MaxAge:   int(cfg.Auth.SessionDuration().Seconds()),
		Expires:  time.Now().Add(cfg.Auth.SessionDuration()),
	})
	return nil
}

func clearAuthCookie(ctx *fiber.Ctx) {
	if ctx == nil {
		return
	}
	ctx.Cookie(&fiber.Cookie{
		Name:     utils.AuthCookieName,
		Value:    "",
		Path:     "/",
		HTTPOnly: true,
		Secure:   strings.EqualFold(ctx.Protocol(), "https"),
		SameSite: fiber.CookieSameSiteLaxMode,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})
}

func sendAPIError(ctx *fiber.Ctx, status int, detail string) error {
	return ctx.Status(status).JSON(fiber.Map{
		"detail": detail,
	})
}
