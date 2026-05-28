package profile

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const (
	baseDirName  = ".claude"
	profilesDir  = ".claude-profiles"
	activeFile   = ".active"
)

type Manager struct {
	BaseDir     string
	ProfilesDir string
}

func NewManager() (*Manager, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	return &Manager{
		BaseDir:     filepath.Join(home, baseDirName),
		ProfilesDir: filepath.Join(home, profilesDir),
	}, nil
}

func (m *Manager) Init(name string) error {
	if err := validateName(name); err != nil {
		return err
	}
	if _, err := os.Stat(m.BaseDir); os.IsNotExist(err) {
		return fmt.Errorf("base config %s does not exist", m.BaseDir)
	}
	dir := m.dir(name)
	if _, err := os.Stat(dir); err == nil {
		return fmt.Errorf("profile %q already exists", name)
	}
	if err := os.MkdirAll(m.ProfilesDir, 0755); err != nil {
		return err
	}
	if err := os.Mkdir(dir, 0755); err != nil {
		return err
	}
	return m.linkAll(name)
}

func (m *Manager) Switch(name string) error {
	dir := m.dir(name)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("profile %q does not exist", name)
	}
	return os.WriteFile(m.activePath(), []byte(name+"\n"), 0644)
}

func (m *Manager) SwitchDefault() error {
	err := os.Remove(m.activePath())
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (m *Manager) Active() (string, bool) {
	data, err := os.ReadFile(m.activePath())
	if err != nil {
		return "", false
	}
	name := strings.TrimSpace(string(data))
	if name == "" {
		return "", false
	}
	return name, true
}

func (m *Manager) Env() string {
	name, ok := m.Active()
	if !ok {
		return "unset CLAUDE_CONFIG_DIR"
	}
	return fmt.Sprintf("export CLAUDE_CONFIG_DIR='%s'", m.dir(name))
}

func (m *Manager) List() ([]string, error) {
	entries, err := os.ReadDir(m.ProfilesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

type FileStatus struct {
	Name     string
	Shared   bool
	LinkTo   string
}

func (m *Manager) Status(name string) ([]FileStatus, error) {
	dir := m.dir(name)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []FileStatus
	for _, e := range entries {
		path := filepath.Join(dir, e.Name())
		info, err := os.Lstat(path)
		if err != nil {
			continue
		}
		s := FileStatus{Name: e.Name()}
		if info.Mode()&fs.ModeSymlink != 0 {
			s.Shared = true
			s.LinkTo, _ = os.Readlink(path)
		}
		out = append(out, s)
	}
	return out, nil
}

func (m *Manager) Override(name, file string) error {
	dir := m.dir(name)
	path := filepath.Join(dir, file)

	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("%q not found in profile %q", file, name)
	}
	if info.Mode()&fs.ModeSymlink == 0 {
		return fmt.Errorf("%q is already overridden", file)
	}

	target, err := os.Readlink(path)
	if err != nil {
		return err
	}
	targetInfo, err := os.Stat(target)
	if err != nil {
		return err
	}

	if err := os.Remove(path); err != nil {
		return err
	}

	if targetInfo.IsDir() {
		return copyDir(target, path)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, targetInfo.Mode())
}

func (m *Manager) Share(name, file string) error {
	dir := m.dir(name)
	path := filepath.Join(dir, file)

	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("%q not found in profile %q", file, name)
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		return fmt.Errorf("%q is already shared", file)
	}

	basePath := filepath.Join(m.BaseDir, file)
	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		return fmt.Errorf("%q not found in base config", file)
	}

	if info.IsDir() {
		if err := os.RemoveAll(path); err != nil {
			return err
		}
	} else {
		if err := os.Remove(path); err != nil {
			return err
		}
	}
	return os.Symlink(basePath, path)
}

func (m *Manager) Sync(name string) ([]string, error) {
	dir := m.dir(name)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, fmt.Errorf("profile %q does not exist", name)
	}
	entries, err := os.ReadDir(m.BaseDir)
	if err != nil {
		return nil, err
	}
	var added []string
	for _, e := range entries {
		dst := filepath.Join(dir, e.Name())
		if _, err := os.Lstat(dst); err == nil {
			continue
		}
		src := filepath.Join(m.BaseDir, e.Name())
		if err := os.Symlink(src, dst); err != nil {
			return added, err
		}
		added = append(added, e.Name())
	}
	return added, nil
}

func (m *Manager) Delete(name string) error {
	active, _ := m.Active()
	if active == name {
		_ = m.SwitchDefault()
	}
	dir := m.dir(name)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("profile %q does not exist", name)
	}
	return os.RemoveAll(dir)
}

func (m *Manager) dir(name string) string {
	return filepath.Join(m.ProfilesDir, name)
}

func (m *Manager) activePath() string {
	return filepath.Join(m.ProfilesDir, activeFile)
}

func (m *Manager) linkAll(name string) error {
	dir := m.dir(name)
	entries, err := os.ReadDir(m.BaseDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		src := filepath.Join(m.BaseDir, e.Name())
		dst := filepath.Join(dir, e.Name())
		if err := os.Symlink(src, dst); err != nil {
			return err
		}
	}
	return nil
}

func validateName(name string) error {
	if name == "" {
		return fmt.Errorf("profile name cannot be empty")
	}
	if strings.ContainsAny(name, "/\\. ") {
		return fmt.Errorf("profile name cannot contain /, \\, ., or spaces")
	}
	return nil
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode())
	})
}
