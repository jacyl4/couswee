package accounts

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var unsafeProfileChars = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

func safeProfileName(value string) string {
	name := strings.Trim(unsafeProfileChars.ReplaceAllString(value, "-"), "-._")
	if name == "" {
		return "account"
	}
	return name
}

type ProfileService struct {
	home string
}

func NewProfileService(home string) ProfileService {
	return ProfileService{home: home}
}

func (p ProfileService) AuthPath(profileName string) string {
	return ManagedAuthPath(p.home, profileName)
}

func (p ProfileService) WriteAuth(profileName string, content []byte) (string, error) {
	path := p.AuthPath(profileName)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", fmt.Errorf("create profile directory: %w", err)
	}
	if err := os.Chmod(filepath.Dir(path), 0o700); err != nil {
		return "", fmt.Errorf("secure profile directory: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, content, 0o600); err != nil {
		return "", fmt.Errorf("write auth file: %w", err)
	}
	if err := os.Chmod(tmp, 0o600); err != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("secure auth file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("replace auth file: %w", err)
	}
	return path, nil
}

func (p ProfileService) RemoveManagedProfile(profileName string) error {
	path := filepath.Dir(p.AuthPath(profileName))
	profilesRoot := ProfilesDir(p.home)
	cleanRoot, err := filepath.Abs(profilesRoot)
	if err != nil {
		return err
	}
	cleanPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if cleanPath == cleanRoot || !strings.HasPrefix(cleanPath, cleanRoot+string(os.PathSeparator)) {
		return fmt.Errorf("refuse to delete unmanaged profile path: %s", path)
	}
	return os.RemoveAll(cleanPath)
}
