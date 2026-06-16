package server

import (
	"context"
	"errors"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"couswee/internal/accounts"
	"couswee/internal/usage"
	"couswee/internal/version"

	"github.com/gofiber/fiber/v2"
)

type Config struct {
	StaticDir string
	StaticFS  fs.FS
	Usage     *usage.Service
}

const DefaultAddr = "0.0.0.0:2199"

type Server struct {
	app                 *fiber.App
	service             *accounts.Service
	usage               *usage.Service
	loginUsageMu        sync.Mutex
	refreshedLoginUsage map[string]struct{}
}

func New(service *accounts.Service, cfg Config) *Server {
	app := fiber.New(fiber.Config{AppName: "couswee"})
	s := &Server{app: app, service: service, usage: cfg.Usage, refreshedLoginUsage: make(map[string]struct{})}
	s.routes(cfg)
	return s
}

func (s *Server) App() *fiber.App { return s.app }

func (s *Server) Listen(addr string) error {
	if addr == "" {
		addr = DefaultAddr
	}
	return s.app.Listen(addr)
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.app.ShutdownWithContext(ctx)
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
	} else if cfg.StaticFS != nil {
		s.app.Get("*", embeddedStaticHandler(cfg.StaticFS))
	} else {
		s.app.Get("/", func(c *fiber.Ctx) error {
			return c.Type("html").SendString(`<html><body><h1>couswee</h1><p>Frontend has not been built yet. Run npm run build.</p></body></html>`)
		})
	}
}

func embeddedStaticHandler(staticFS fs.FS) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if strings.HasPrefix(c.Path(), "/api/") {
			return c.SendStatus(http.StatusNotFound)
		}

		name := strings.TrimPrefix(c.Path(), "/")
		if name == "" {
			name = "index.html"
		}
		if !fs.ValidPath(name) {
			return c.SendStatus(http.StatusNotFound)
		}

		if served, err := sendEmbeddedFile(c, staticFS, name); err != nil {
			return err
		} else if served {
			return nil
		}
		return sendEmbeddedFallback(c, staticFS)
	}
}

func sendEmbeddedFile(c *fiber.Ctx, staticFS fs.FS, name string) (bool, error) {
	info, err := fs.Stat(staticFS, name)
	if err == nil && info.IsDir() {
		name = filepath.ToSlash(filepath.Join(name, "index.html"))
	}

	file, err := staticFS.Open(name)
	if err != nil {
		return false, nil
	}
	defer file.Close()

	if ext := filepath.Ext(name); ext != "" {
		if contentType := mime.TypeByExtension(ext); contentType != "" {
			c.Set(fiber.HeaderContentType, contentType)
		}
	}
	return true, c.SendStream(file)
}

func sendEmbeddedFallback(c *fiber.Ctx, staticFS fs.FS) error {
	file, err := staticFS.Open("fallback.html")
	if err != nil {
		return c.SendStatus(http.StatusNotFound)
	}
	defer file.Close()
	return c.Type("html").SendStream(file)
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
	if s.usage != nil {
		if s.usage.RefreshAccountWithReason(context.Background(), account.ID, usage.RefreshReasonAccountAdded) {
			account = s.findAccount(account.ID, account)
		}
	}
	return c.Status(http.StatusCreated).JSON(account)
}

func (s *Server) findAccount(selector string, fallback accounts.Account) accounts.Account {
	for _, account := range s.service.Accounts() {
		if account.ID == selector || account.ProfileName == selector {
			return account
		}
	}
	return fallback
}

func (s *Server) deleteAccounts(c *fiber.Ctx) error {
	var req struct {
		ProfileNames []string `json:"profile_names"`
		IDs          []string `json:"ids"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	selectors := append([]string{}, req.ProfileNames...)
	selectors = append(selectors, req.IDs...)
	deleted, err := s.service.DeleteSelectors(selectors)
	if err != nil {
		if errors.Is(err, accounts.ErrAccountNotFound) {
			return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "account not found"})
		}
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	if s.usage != nil {
		s.usage.PruneCurrentAccounts()
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
		ID      string `json:"id"`
		Profile string `json:"profile_name"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	selector := req.Profile
	if selector == "" {
		selector = req.ID
	}
	if selector == "" {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "profile_name or id is required"})
	}
	account, err := s.service.SwitchSelector(selector)
	if err != nil {
		if errors.Is(err, accounts.ErrAccountNotFound) {
			return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "account not found"})
		}
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	if s.usage != nil {
		s.usage.RefreshAccountWithReason(context.Background(), account.ID, usage.RefreshReasonAccountSwitch)
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
	s.refreshLoginUsage(session)
	return c.JSON(session)
}

func (s *Server) refreshLoginUsage(session accounts.LoginSession) {
	if s.usage == nil || session.Status != accounts.LoginStatusSucceeded || strings.TrimSpace(session.AccountID) == "" {
		return
	}
	sessionID := strings.TrimSpace(session.ID)
	if sessionID == "" {
		return
	}
	s.loginUsageMu.Lock()
	if _, ok := s.refreshedLoginUsage[sessionID]; ok {
		s.loginUsageMu.Unlock()
		return
	}
	s.refreshedLoginUsage[sessionID] = struct{}{}
	s.loginUsageMu.Unlock()

	if s.usage.RefreshAccountWithReason(context.Background(), session.AccountID, usage.RefreshReasonLoginSuccess) {
		return
	}
	s.loginUsageMu.Lock()
	delete(s.refreshedLoginUsage, sessionID)
	s.loginUsageMu.Unlock()
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
