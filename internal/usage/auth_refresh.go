package usage

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const authRefreshSkew = 2 * time.Minute

type AuthRefresher interface {
	RefreshCodexAuth(ctx context.Context, authPath string) error
}

type CodexCLIAuthRefresher struct {
	Command string
	Home    string
	Timeout time.Duration
}

func (r CodexCLIAuthRefresher) RefreshCodexAuth(ctx context.Context, authPath string) error {
	authPath = ExpandHome(authPath)
	if strings.TrimSpace(authPath) == "" {
		return fmt.Errorf("codex auth path is empty")
	}
	command := strings.TrimSpace(r.Command)
	if command == "" {
		command = "codex"
	}
	timeout := r.Timeout
	if timeout <= 0 {
		timeout = DefaultAuthRefreshTimeout
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, command, "doctor")
	cmd.Env = codexAuthRefreshEnv(os.Environ(), r.Home, filepath.Dir(authPath))
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("refresh codex auth timed out after %s: %w", timeout, ctx.Err())
		}
		detail := sanitizeCommandOutput(output.String())
		if detail == "" {
			return fmt.Errorf("refresh codex auth with %s doctor: %w", command, err)
		}
		return fmt.Errorf("refresh codex auth with %s doctor: %w: %s", command, err, detail)
	}
	return nil
}

func codexAuthRefreshEnv(base []string, home, codexHome string) []string {
	env := make([]string, 0, len(base)+2)
	for _, value := range base {
		if strings.HasPrefix(value, "HOME=") || strings.HasPrefix(value, "CODEX_HOME=") {
			continue
		}
		env = append(env, value)
	}
	home = strings.TrimSpace(home)
	if home == "" {
		if detected, err := os.UserHomeDir(); err == nil {
			home = detected
		}
	}
	if home != "" {
		env = append(env, "HOME="+home)
	}
	return append(env, "CODEX_HOME="+codexHome)
}

func sanitizeCommandOutput(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	lines := strings.Split(value, "\n")
	kept := lines
	if len(kept) > 4 {
		kept = kept[len(kept)-4:]
	}
	for i, line := range kept {
		kept[i] = strings.TrimSpace(line)
	}
	return strings.Join(kept, " | ")
}

func codexAuthNeedsRefresh(auth CodexAuth, now time.Time) bool {
	if now.IsZero() {
		now = time.Now()
	}
	if !auth.AccessTokenExpiresAt.IsZero() {
		return !auth.AccessTokenExpiresAt.After(now.Add(authRefreshSkew))
	}
	return false
}

func jwtExpiresAt(token string) time.Time {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return time.Time{}
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		payload, err = base64.URLEncoding.DecodeString(parts[1])
		if err != nil {
			return time.Time{}
		}
	}
	var claims struct {
		Exp json.Number `json:"exp"`
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	if err := decoder.Decode(&claims); err != nil {
		return time.Time{}
	}
	exp, err := claims.Exp.Int64()
	if err != nil || exp <= 0 {
		return time.Time{}
	}
	return time.Unix(exp, 0).UTC()
}

func parseAuthTime(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return t
	}
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t
	}
	return time.Time{}
}
