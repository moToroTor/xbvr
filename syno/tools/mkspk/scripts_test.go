package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// DSM's upgrader requires per-action scripts beside the installer
// dispatcher; a package without preupgrade/postupgrade fails to install
// with error 261. The pack list must always contain all six.
func TestActionScriptsPacked(t *testing.T) {
	dir := spkDir(t)
	read := func(rel string) []byte {
		b, err := os.ReadFile(filepath.Join(dir, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		return b
	}
	got := dsmActionScripts(read)
	want := []string{
		"preinst", "postinst", "preuninst", "postuninst", "preupgrade", "postupgrade",
	}
	if len(got) != len(want) {
		t.Fatalf("packed %d action scripts, want %d", len(got), len(want))
	}
	for i, a := range want {
		tf := got[i]
		if tf.Name != "scripts/"+a {
			t.Errorf("entry %d name = %q, want scripts/%s", i, tf.Name, a)
		}
		if tf.Mode != 0o755 {
			t.Errorf("%s mode = %o, want 755", a, tf.Mode)
		}
		body := string(tf.Data)
		if !strings.HasPrefix(body, "#!/bin/sh\n") {
			t.Errorf("%s missing sh shebang", a)
		}
		if !strings.Contains(body, `/installer" "`+a+`"`) {
			t.Errorf("%s does not delegate to installer %s", a, a)
		}
	}
}

// Each stub must invoke the installer dispatcher with its own action.
func TestStubDelegation(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}
	stage := t.TempDir()
	for _, a := range []string{
		"preinst", "postinst", "preuninst", "postuninst", "preupgrade", "postupgrade",
	} {
		raw, err := os.ReadFile(filepath.Join(spkDir(t), "scripts", a))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(stage, a), raw, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	fake := "#!/bin/sh\necho \"$1\" >> \"$0.seen\"\n"
	if err := os.WriteFile(filepath.Join(stage, "installer"), []byte(fake), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, a := range []string{
		"preinst", "postinst", "preuninst", "postuninst", "preupgrade", "postupgrade",
	} {
		cmd := exec.Command("sh", "./"+a)
		cmd.Dir = stage
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("stub %s failed: %v\n%s", a, err, out)
		}
	}
	seen, err := os.ReadFile(filepath.Join(stage, "installer.seen"))
	if err != nil {
		t.Fatalf("fake installer never ran: %v", err)
	}
	for _, a := range []string{
		"preinst", "postinst", "preuninst", "postuninst", "preupgrade", "postupgrade",
	} {
		if !strings.Contains(string(seen), a+"\n") {
			t.Errorf("installer never received action %q (got %q)", a, seen)
		}
	}
}

// The installer must propagate hook failures instead of masking them
// behind exit 0, while hooks that don't exist must still succeed.
func TestInstallerPropagatesFailure(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}
	scripts := filepath.Join(spkDir(t), "scripts")

	runInstaller := func(args []string, pkgvar string) error {
		cmd := exec.Command("sh", append([]string{"./installer"}, args...)...)
		cmd.Dir = scripts
		cmd.Env = []string{"SYNOPKG_PKGVAR=" + pkgvar, "PATH=/usr/bin:/bin"}
		return cmd.Run()
	}

	// preinst has no hook: must succeed.
	if err := runInstaller([]string{"preinst"}, t.TempDir()); err != nil {
		t.Errorf("preinst without hook failed: %v", err)
	}
	// postinst pointed at a regular file (not a dir): mkdir and the .env
	// write both fail, and the installer must report failure.
	blocker := filepath.Join(t.TempDir(), "afile")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runInstaller([]string{"postinst"}, blocker); err == nil {
		t.Error("postinst with unwritable PKGVAR succeeded; want failure")
	}
}
