package profile

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	tmp := t.TempDir()
	base := filepath.Join(tmp, ".claude")
	profiles := filepath.Join(tmp, ".claude-profiles")
	if err := os.Mkdir(base, 0755); err != nil {
		t.Fatal(err)
	}
	// seed base with a file and a directory
	os.WriteFile(filepath.Join(base, "CLAUDE.md"), []byte("# base"), 0644)
	os.WriteFile(filepath.Join(base, "settings.json"), []byte(`{"v":1}`), 0644)
	sub := filepath.Join(base, "projects")
	os.Mkdir(sub, 0755)
	os.WriteFile(filepath.Join(sub, "memo.md"), []byte("memo"), 0644)
	return &Manager{BaseDir: base, ProfilesDir: profiles}
}

func isSymlink(path string) bool {
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}
	return info.Mode()&fs.ModeSymlink != 0
}

// --- Init ---

func TestInit(t *testing.T) {
	m := newTestManager(t)
	if err := m.Init("work"); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(m.ProfilesDir, "work")
	// profile dir should exist
	if _, err := os.Stat(dir); err != nil {
		t.Fatal("profile dir not created")
	}
	// all entries should be symlinks
	for _, name := range []string{"CLAUDE.md", "settings.json", "projects"} {
		p := filepath.Join(dir, name)
		if !isSymlink(p) {
			t.Errorf("%s should be a symlink", name)
		}
		target, _ := os.Readlink(p)
		if target != filepath.Join(m.BaseDir, name) {
			t.Errorf("%s points to %s, want %s", name, target, filepath.Join(m.BaseDir, name))
		}
	}
}

func TestInitDuplicate(t *testing.T) {
	m := newTestManager(t)
	m.Init("work")
	if err := m.Init("work"); err == nil {
		t.Fatal("expected error on duplicate init")
	}
}

func TestInitInvalidName(t *testing.T) {
	m := newTestManager(t)
	for _, name := range []string{"", "a/b", "a b", ".hidden", "a\\b"} {
		if err := m.Init(name); err == nil {
			t.Errorf("expected error for name %q", name)
		}
	}
}

func TestInitNoBase(t *testing.T) {
	m := &Manager{BaseDir: "/nonexistent", ProfilesDir: "/tmp/ccp-test-profiles"}
	if err := m.Init("x"); err == nil {
		t.Fatal("expected error when base does not exist")
	}
}

// --- Switch / Active ---

func TestSwitchAndActive(t *testing.T) {
	m := newTestManager(t)
	m.Init("work")
	m.Init("personal")

	if err := m.Switch("work"); err != nil {
		t.Fatal(err)
	}
	name, ok := m.Active()
	if !ok || name != "work" {
		t.Fatalf("got %q, want %q", name, "work")
	}

	m.Switch("personal")
	name, _ = m.Active()
	if name != "personal" {
		t.Fatalf("got %q, want %q", name, "personal")
	}
}

func TestSwitchNonexistent(t *testing.T) {
	m := newTestManager(t)
	if err := m.Switch("nope"); err == nil {
		t.Fatal("expected error")
	}
}

func TestSwitchDefault(t *testing.T) {
	m := newTestManager(t)
	m.Init("work")
	m.Switch("work")
	m.SwitchDefault()
	_, ok := m.Active()
	if ok {
		t.Fatal("expected no active profile")
	}
}

func TestSwitchDefaultIdempotent(t *testing.T) {
	m := newTestManager(t)
	// should not error even if no active file exists
	if err := m.SwitchDefault(); err != nil {
		t.Fatal(err)
	}
}

// --- Env ---

func TestEnv(t *testing.T) {
	m := newTestManager(t)
	m.Init("work")

	// no active profile
	if got := m.Env(); got != "unset CLAUDE_CONFIG_DIR" {
		t.Fatalf("got %q", got)
	}
	// with active profile
	m.Switch("work")
	want := "export CLAUDE_CONFIG_DIR='" + filepath.Join(m.ProfilesDir, "work") + "'"
	if got := m.Env(); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// --- List ---

func TestList(t *testing.T) {
	m := newTestManager(t)
	names, err := m.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 0 {
		t.Fatalf("expected empty list, got %v", names)
	}

	m.Init("b-profile")
	m.Init("a-profile")
	names, _ = m.List()
	if len(names) != 2 {
		t.Fatalf("expected 2, got %d", len(names))
	}
	// ReadDir returns sorted order
	if names[0] != "a-profile" || names[1] != "b-profile" {
		t.Fatalf("unexpected order: %v", names)
	}
}

func TestListBeforeProfilesDir(t *testing.T) {
	m := &Manager{BaseDir: "/tmp", ProfilesDir: "/nonexistent-ccp-test"}
	names, err := m.List()
	if err != nil {
		t.Fatal(err)
	}
	if names != nil {
		t.Fatalf("expected nil, got %v", names)
	}
}

// --- Status ---

func TestStatus(t *testing.T) {
	m := newTestManager(t)
	m.Init("work")

	statuses, err := m.Status("work")
	if err != nil {
		t.Fatal(err)
	}
	if len(statuses) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(statuses))
	}
	for _, s := range statuses {
		if !s.Shared {
			t.Errorf("%s should be shared", s.Name)
		}
	}
}

// --- Override ---

func TestOverrideFile(t *testing.T) {
	m := newTestManager(t)
	m.Init("work")

	if err := m.Override("work", "CLAUDE.md"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(m.ProfilesDir, "work", "CLAUDE.md")
	if isSymlink(path) {
		t.Fatal("should no longer be a symlink")
	}
	data, _ := os.ReadFile(path)
	if string(data) != "# base" {
		t.Fatalf("content mismatch: %q", data)
	}
}

func TestOverrideDir(t *testing.T) {
	m := newTestManager(t)
	m.Init("work")

	if err := m.Override("work", "projects"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(m.ProfilesDir, "work", "projects")
	if isSymlink(path) {
		t.Fatal("should no longer be a symlink")
	}
	// nested file should be copied
	data, err := os.ReadFile(filepath.Join(path, "memo.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "memo" {
		t.Fatalf("content mismatch: %q", data)
	}
}

func TestOverrideAlreadyOverridden(t *testing.T) {
	m := newTestManager(t)
	m.Init("work")
	m.Override("work", "CLAUDE.md")
	if err := m.Override("work", "CLAUDE.md"); err == nil {
		t.Fatal("expected error on double override")
	}
}

func TestOverrideNonexistent(t *testing.T) {
	m := newTestManager(t)
	m.Init("work")
	if err := m.Override("work", "nope.txt"); err == nil {
		t.Fatal("expected error")
	}
}

// --- Share ---

func TestShare(t *testing.T) {
	m := newTestManager(t)
	m.Init("work")
	m.Override("work", "CLAUDE.md")

	if err := m.Share("work", "CLAUDE.md"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(m.ProfilesDir, "work", "CLAUDE.md")
	if !isSymlink(path) {
		t.Fatal("should be a symlink again")
	}
	target, _ := os.Readlink(path)
	if target != filepath.Join(m.BaseDir, "CLAUDE.md") {
		t.Fatalf("symlink target: %s", target)
	}
}

func TestShareDir(t *testing.T) {
	m := newTestManager(t)
	m.Init("work")
	m.Override("work", "projects")

	if err := m.Share("work", "projects"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(m.ProfilesDir, "work", "projects")
	if !isSymlink(path) {
		t.Fatal("should be a symlink again")
	}
}

func TestShareAlreadyShared(t *testing.T) {
	m := newTestManager(t)
	m.Init("work")
	if err := m.Share("work", "CLAUDE.md"); err == nil {
		t.Fatal("expected error")
	}
}

// --- Sync ---

func TestSync(t *testing.T) {
	m := newTestManager(t)
	m.Init("work")

	// add a new file to base
	os.WriteFile(filepath.Join(m.BaseDir, "new.json"), []byte("{}"), 0644)

	added, err := m.Sync("work")
	if err != nil {
		t.Fatal(err)
	}
	if len(added) != 1 || added[0] != "new.json" {
		t.Fatalf("unexpected added: %v", added)
	}
	path := filepath.Join(m.ProfilesDir, "work", "new.json")
	if !isSymlink(path) {
		t.Fatal("should be a symlink")
	}
}

func TestSyncNoChanges(t *testing.T) {
	m := newTestManager(t)
	m.Init("work")
	added, err := m.Sync("work")
	if err != nil {
		t.Fatal(err)
	}
	if len(added) != 0 {
		t.Fatalf("expected no additions, got %v", added)
	}
}

func TestSyncNonexistent(t *testing.T) {
	m := newTestManager(t)
	if _, err := m.Sync("nope"); err == nil {
		t.Fatal("expected error")
	}
}

// --- Delete ---

func TestDelete(t *testing.T) {
	m := newTestManager(t)
	m.Init("work")
	m.Switch("work")

	if err := m.Delete("work"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(m.ProfilesDir, "work")); !os.IsNotExist(err) {
		t.Fatal("profile dir should be deleted")
	}
	// active should be cleared
	_, ok := m.Active()
	if ok {
		t.Fatal("active should be cleared after deleting active profile")
	}
}

func TestDeleteNonActive(t *testing.T) {
	m := newTestManager(t)
	m.Init("work")
	m.Init("personal")
	m.Switch("personal")

	m.Delete("work")
	name, ok := m.Active()
	if !ok || name != "personal" {
		t.Fatalf("active should still be personal, got %q", name)
	}
}

func TestDeleteNonexistent(t *testing.T) {
	m := newTestManager(t)
	if err := m.Delete("nope"); err == nil {
		t.Fatal("expected error")
	}
}

// --- Override then edit doesn't affect base ---

func TestOverrideIsolation(t *testing.T) {
	m := newTestManager(t)
	m.Init("work")
	m.Override("work", "CLAUDE.md")

	// modify the overridden file
	path := filepath.Join(m.ProfilesDir, "work", "CLAUDE.md")
	os.WriteFile(path, []byte("# modified"), 0644)

	// base should be untouched
	base, _ := os.ReadFile(filepath.Join(m.BaseDir, "CLAUDE.md"))
	if string(base) != "# base" {
		t.Fatalf("base was modified: %q", base)
	}
}

// --- Shared file reflects base changes ---

func TestSharedReflectsBaseChanges(t *testing.T) {
	m := newTestManager(t)
	m.Init("work")

	// modify base
	os.WriteFile(filepath.Join(m.BaseDir, "CLAUDE.md"), []byte("# updated"), 0644)

	// profile should see the change via symlink
	data, _ := os.ReadFile(filepath.Join(m.ProfilesDir, "work", "CLAUDE.md"))
	if string(data) != "# updated" {
		t.Fatalf("expected updated content, got %q", data)
	}
}
