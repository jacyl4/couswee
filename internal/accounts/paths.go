package accounts

import (
	"os"
	"path/filepath"
	"strings"
)

func DBPath(home string) string {
	return filepath.Join(home, ".couswee", "couswee.db")
}

func ProfilesDir(home string) string {
	return filepath.Join(home, ".couswee", "profiles")
}

func ManagedAuthPath(home, profileName string) string {
	return filepath.Join(ProfilesDir(home), safeProfileName(profileName), "auth.json")
}

func IsolatedCodexHomePath(home string) string {
	return filepath.Join(home, ".codex")
}

func CodexHomePath(home string) string {
	if codexHome := strings.TrimSpace(os.Getenv("CODEX_HOME")); codexHome != "" {
		return ExpandUserPath(codexHome, home)
	}
	return IsolatedCodexHomePath(home)
}

func CodexAuthPath(home string) string {
	return filepath.Join(CodexHomePath(home), "auth.json")
}

func ExpandUserPath(path string, home string) string {
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, strings.TrimPrefix(path, "~/"))
	}
	if strings.HasPrefix(path, "$HOME/") {
		return filepath.Join(home, strings.TrimPrefix(path, "$HOME/"))
	}
	if strings.HasPrefix(path, "${HOME}/") {
		return filepath.Join(home, strings.TrimPrefix(path, "${HOME}/"))
	}
	return os.ExpandEnv(path)
}
