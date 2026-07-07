package accounts

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"
)

const authExpirySkew = 2 * time.Minute

func enrichAuthState(account Account, home string, now time.Time) Account {
	if now.IsZero() {
		now = time.Now()
	}
	path := ExpandUserPath(account.AuthPath, home)
	data, err := os.ReadFile(path)
	if err != nil {
		account.AuthStatus = "missing"
		account.AuthError = fmt.Sprintf("read auth file: %v", err)
		return account
	}
	if lastRefresh := authLastRefresh(data); !lastRefresh.IsZero() {
		account.AuthLastRefresh = lastRefresh.UTC().Format(time.RFC3339)
	}
	exp, err := authAccessExpiresAt(data)
	if err != nil {
		account.AuthStatus = "invalid"
		account.AuthError = err.Error()
		return account
	}
	if exp.IsZero() {
		account.AuthStatus = "unknown"
		return account
	}
	account.AuthExpiresAt = exp.UTC().Format(time.RFC3339)
	if !exp.After(now.Add(authExpirySkew)) {
		account.AuthStatus = "expired"
		account.AuthExpired = true
		return account
	}
	account.AuthStatus = "ready"
	return account
}

func authLastRefresh(data []byte) time.Time {
	var raw struct {
		LastRefresh string `json:"last_refresh"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return time.Time{}
	}
	return parseAuthTimestamp(raw.LastRefresh)
}

func authAccessExpiresAt(data []byte) (time.Time, error) {
	var raw struct {
		Tokens struct {
			AccessToken string `json:"access_token"`
		} `json:"tokens"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return time.Time{}, fmt.Errorf("parse auth file: %w", err)
	}
	token := strings.TrimSpace(raw.Tokens.AccessToken)
	if token == "" {
		return time.Time{}, fmt.Errorf("auth file has no tokens.access_token")
	}
	return jwtExpiresAt(token), nil
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

func parseAuthTimestamp(value string) time.Time {
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

func mergeCodexAuthConfigFields(active, target []byte) ([]byte, bool, error) {
	if authAccountID(active) == "" || authAccountID(target) == "" {
		return nil, false, nil
	}
	var activeFields map[string]json.RawMessage
	if err := json.Unmarshal(active, &activeFields); err != nil {
		return nil, false, fmt.Errorf("parse active auth file: %w", err)
	}
	var targetFields map[string]json.RawMessage
	if err := json.Unmarshal(target, &targetFields); err != nil {
		return nil, false, fmt.Errorf("parse managed auth file: %w", err)
	}
	changed := false
	for key, value := range activeFields {
		if isAccountSpecificAuthField(key) {
			continue
		}
		if isSensitiveConfigField(key) {
			if !isEmptyAuthConfigValue(value) {
				continue
			}
			if existing, ok := targetFields[key]; ok && !isEmptyAuthConfigValue(existing) {
				continue
			}
		}
		if !jsonRawEqual(targetFields[key], value) {
			targetFields[key] = append(json.RawMessage(nil), value...)
			changed = true
		}
	}
	if !changed {
		return nil, false, nil
	}
	next, err := json.MarshalIndent(targetFields, "", "  ")
	if err != nil {
		return nil, false, fmt.Errorf("encode managed auth file: %w", err)
	}
	return append(next, '\n'), true, nil
}

func isAccountSpecificAuthField(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "tokens", "last_refresh":
		return true
	default:
		return false
	}
}

func isSensitiveConfigField(key string) bool {
	lower := strings.ToLower(strings.TrimSpace(key))
	return strings.Contains(lower, "api_key") ||
		strings.Contains(lower, "secret") ||
		strings.Contains(lower, "credential")
}

func isEmptyAuthConfigValue(value json.RawMessage) bool {
	trimmed := bytes.TrimSpace(value)
	if bytes.Equal(trimmed, []byte("null")) || bytes.Equal(trimmed, []byte(`""`)) || bytes.Equal(trimmed, []byte("{}")) {
		return true
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &obj); err == nil && len(obj) == 0 {
		return true
	}
	return false
}

func jsonRawEqual(a, b json.RawMessage) bool {
	var av any
	var bv any
	if json.Unmarshal(a, &av) != nil || json.Unmarshal(b, &bv) != nil {
		return bytes.Equal(bytes.TrimSpace(a), bytes.TrimSpace(b))
	}
	return reflect.DeepEqual(av, bv)
}

func writeFileAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create auth directory: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write temporary auth file: %w", err)
	}
	if err := os.Chmod(tmp, 0o600); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("secure temporary auth file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("replace auth file: %w", err)
	}
	return nil
}
