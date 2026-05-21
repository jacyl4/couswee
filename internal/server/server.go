package server

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"couswee/internal/accounts"
	"couswee/internal/usage"
	"couswee/internal/version"

	"github.com/gofiber/fiber/v2"
)

type Config struct {
	StaticDir string
	Usage     *usage.Service
}

type Server struct {
	app     *fiber.App
	service *accounts.Service
	usage   *usage.Service
}

func New(service *accounts.Service, cfg Config) *Server {
	app := fiber.New(fiber.Config{AppName: "couswee"})
	s := &Server{app: app, service: service, usage: cfg.Usage}
	s.routes(cfg)
	return s
}

func (s *Server) App() *fiber.App { return s.app }

func (s *Server) Listen(addr string) error {
	if addr == "" {
		addr = "127.0.0.1:2199"
	}
	return s.app.Listen(addr)
}

func (s *Server) routes(cfg Config) {
	s.app.Get("/api/accounts", s.getAccounts)
	s.app.Post("/api/accounts", s.postAccount)
	s.app.Delete("/api/accounts", s.deleteAccounts)
	s.app.Patch("/api/accounts/:id", s.patchAccount)
	s.app.Get("/api/current", s.getCurrent)
	s.app.Post("/api/switch", s.postSwitch)
	s.app.Post("/api/codex/login/start", s.postLoginStart)
	s.app.Post("/api/codex/login/oauth/start", s.postLoginStart)
	s.app.Post("/api/codex/login/device/start", s.postLoginStart)
	s.app.Get("/api/codex/login/:session_id", s.getLoginSession)
	s.app.Post("/api/codex/login/:session_id/cancel", s.postLoginCancel)
	s.app.Get("/api/codex/usage", s.getCodexUsage)
	s.app.Get("/api/version", s.getVersion)
	s.app.Get("/api/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"ok": true, "time": time.Now().Format(time.RFC3339)})
	})

	staticDir := cfg.StaticDir
	if staticDir == "" {
		staticDir = filepath.Join("web", "dist")
	}
	if info, err := os.Stat(staticDir); err == nil && info.IsDir() {
		s.app.Static("/", staticDir)
		fallbackPath := filepath.Join(staticDir, "fallback.html")
		if info, err := os.Stat(fallbackPath); err == nil && !info.IsDir() {
			s.app.Get("*", func(c *fiber.Ctx) error {
				if strings.HasPrefix(c.Path(), "/api/") {
					return c.SendStatus(http.StatusNotFound)
				}
				return c.Type("html").SendFile(fallbackPath)
			})
		}
	} else {
		s.app.Get("/", func(c *fiber.Ctx) error {
			return c.Type("html").SendString(`<html><body><h1>couswee</h1><p>Frontend has not been built yet. Run npm run build.</p></body></html>`)
		})
	}
}

func (s *Server) getAccounts(c *fiber.Ctx) error {
	return c.JSON(s.service.Accounts())
}

func (s *Server) postAccount(c *fiber.Ctx) error {
	var req accounts.Account
	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	account, err := s.service.Add(req)
	if err != nil {
		switch {
		case errors.Is(err, accounts.ErrInvalidAccount):
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "nickname and auth_path are required"})
		case errors.Is(err, accounts.ErrDuplicateAccount):
			return c.Status(http.StatusConflict).JSON(fiber.Map{"error": "account already exists"})
		default:
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
	}
	return c.Status(http.StatusCreated).JSON(account)
}

func (s *Server) deleteAccounts(c *fiber.Ctx) error {
	var req struct {
		Nicknames []string `json:"nicknames"`
		IDs       []string `json:"ids"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	selectors := append([]string{}, req.Nicknames...)
	selectors = append(selectors, req.IDs...)
	deleted, err := s.service.DeleteSelectors(selectors)
	if err != nil {
		if errors.Is(err, accounts.ErrAccountNotFound) {
			return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "account not found"})
		}
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"deleted": deleted})
}

func (s *Server) getCurrent(c *fiber.Ctx) error {
	account, err := s.service.Current()
	if err != nil {
		if errors.Is(err, accounts.ErrNoActiveAccount) {
			return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "no active account"})
		}
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(account)
}

func (s *Server) postSwitch(c *fiber.Ctx) error {
	var req struct {
		Nickname string `json:"nickname"`
		ID       string `json:"id"`
		Profile  string `json:"profile_name"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	selector := req.Nickname
	if selector == "" {
		selector = req.ID
	}
	if selector == "" {
		selector = req.Profile
	}
	if selector == "" {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "nickname or id is required"})
	}
	account, err := s.service.SwitchSelector(selector)
	if err != nil {
		if errors.Is(err, accounts.ErrAccountNotFound) {
			return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "account not found"})
		}
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	if s.usage != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		s.usage.RefreshAccount(ctx, account.ID)
	}
	return c.JSON(account)
}

func (s *Server) getCodexUsage(c *fiber.Ctx) error {
	if s.usage == nil {
		return c.JSON([]usage.UsageRecord{})
	}
	return c.JSON(s.usage.Records())
}

func (s *Server) getVersion(c *fiber.Ctx) error {
	return c.JSON(version.Current())
}

func (s *Server) patchAccount(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "account id is required"})
	}
	var req accounts.Account
	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	account, err := s.service.UpdateAccount(id, req)
	if err != nil {
		if errors.Is(err, accounts.ErrAccountNotFound) {
			return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "account not found"})
		}
		if errors.Is(err, accounts.ErrDuplicateAccount) {
			return c.Status(http.StatusConflict).JSON(fiber.Map{"error": "account already exists"})
		}
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(account)
}

func (s *Server) postLoginStart(c *fiber.Ctx) error {
	session, err := s.service.StartLogin()
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(http.StatusCreated).JSON(session)
}

func (s *Server) getLoginSession(c *fiber.Ctx) error {
	session, err := s.service.LoginSession(c.Params("session_id"))
	if err != nil {
		if errors.Is(err, accounts.ErrAccountNotFound) {
			return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "login session not found"})
		}
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(session)
}

func (s *Server) postLoginCancel(c *fiber.Ctx) error {
	session, err := s.service.CancelLoginSession(c.Params("session_id"))
	if err != nil {
		if errors.Is(err, accounts.ErrAccountNotFound) {
			return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "login session not found"})
		}
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(session)
}
