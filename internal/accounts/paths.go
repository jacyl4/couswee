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

func CodexAuthPath(home string) string {
	return filepath.Join(home, ".codex", "auth.json")
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
