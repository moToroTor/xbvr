package main

import (
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"testing"
)

// spkDir locates the packaging skeleton relative to this test.
func spkDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join("..", "..", "spk")
	if _, err := os.Stat(filepath.Join(dir, "INFO")); err != nil {
		t.Skipf("spk skeleton not found at %s: %v", dir, err)
	}
	return dir
}

type wizardStep struct {
	StepTitle string `json:"step_title"`
	Items     []struct {
		Type     string `json:"type"`
		Desc     string `json:"desc"`
		SubItems []struct {
			Key  string `json:"key"`
			Desc string `json:"desc"`
			// DefaultValue is any: checkboxes emit bare booleans.
			DefaultValue any `json:"defaultValue"`
		} `json:"subitems"`
	} `json:"items"`
}

func defaultString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case bool:
		if t {
			return "true"
		}
		return "false"
	case string:
		return t
	default:
		return ""
	}
}

// The wizard DSM renders must be an array of steps with subitems keyed
// wizard_*, covering port, MariaDB settings and video directories.
func assertWizardShape(t *testing.T, steps []wizardStep) map[string]string {
	t.Helper()
	if len(steps) == 0 {
		t.Fatal("wizard has no steps")
	}
	seen := map[string]string{}
	for _, s := range steps {
		if s.StepTitle == "" {
			t.Error("step without step_title")
		}
		for _, it := range s.Items {
			if it.Type != "" && len(it.SubItems) == 0 {
				continue // info-only item (e.g. permissions page)
			}
			for _, sub := range it.SubItems {
				if !strings.HasPrefix(sub.Key, "wizard_") {
					t.Errorf("subitem key %q lacks wizard_ prefix", sub.Key)
				}
				if sub.Desc == "" {
					t.Errorf("subitem %q has no desc", sub.Key)
				}
				if _, dup := seen[sub.Key]; dup {
					t.Errorf("duplicate wizard key %q", sub.Key)
				}
				seen[sub.Key] = defaultString(sub.DefaultValue)
			}
		}
	}
	for _, want := range []string{
		"wizard_port", "wizard_video_share", "wizard_video_subdirs",
		"wizard_db_use", "wizard_db_host", "wizard_db_port",
		"wizard_db_name", "wizard_db_user", "wizard_db_pass",
		"wizard_db_root_pass",
	} {
		if _, ok := seen[want]; !ok {
			t.Errorf("wizard key %q not asked for", want)
		}
	}
	return seen
}

// runWizard executes a WIZARD_UIFILES/*.sh generator the way DSM does and
// returns the parsed wizard JSON it writes to the temp logfile.
func runWizard(t *testing.T, script, pkgvar string, env map[string]string) []wizardStep {
	t.Helper()
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}
	out := filepath.Join(t.TempDir(), "wizard.json")
	cmd := exec.Command("bash", script)
	cmd.Env = []string{
		"SYNOPKG_PKGVAR=" + pkgvar,
		"SYNOPKG_TEMP_LOGFILE=" + out,
		"PATH=/usr/bin:/bin",
	}
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	if combined, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s failed: %v\n%s", script, err, combined)
	}
	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("%s wrote no wizard JSON: %v", script, err)
	}
	var steps []wizardStep
	if err := json.Unmarshal(raw, &steps); err != nil {
		t.Fatalf("%s output is not valid JSON: %v\n%s", script, err, raw)
	}
	return steps
}

func TestInstallWizardDefaults(t *testing.T) {
	steps := runWizard(t,
		filepath.Join(spkDir(t), "WIZARD_UIFILES", "install_uifile.sh"),
		t.TempDir(), nil)
	defaults := assertWizardShape(t, steps)
	if defaults["wizard_port"] != "9999" {
		t.Errorf("wizard_port default = %q, want 9999", defaults["wizard_port"])
	}
	if defaults["wizard_db_port"] != "3307" {
		t.Errorf("wizard_db_port default = %q, want DSM MariaDB 3307", defaults["wizard_db_port"])
	}
	if defaults["wizard_db_use"] != "true" {
		t.Errorf("wizard_db_use default = %q, want true (MariaDB opt-out)", defaults["wizard_db_use"])
	}
	if defaults["wizard_db_host"] != "127.0.0.1" {
		t.Errorf("wizard_db_host default = %q, want 127.0.0.1", defaults["wizard_db_host"])
	}
}

// The upgrade wizard pre-fills port, MariaDB parts (parsed back out of
// DATABASE_URL) and video dirs, byte-exact through quotes and backslashes.
func TestUpgradeWizardPrefill(t *testing.T) {
	pkgvar := t.TempDir()
	tricky := `s3cr#t 'q' and "dbl" and \ backslash`
	dotenv := "XBVR_APPDIR='/var/packages/xbvr/var'\n" +
		"XBVR_WEB_PORT='9998'\n" +
		"DATABASE_URL='mysql://xbvr:" + strings.ReplaceAll(tricky, `'`, `'\''`) + "@127.0.0.1:3307/xbvr'\n"
	volumes := "# reminder\n/volume1/video\n/volume2/more vr\n"
	if err := os.WriteFile(filepath.Join(pkgvar, ".env"), []byte(dotenv), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgvar, "volumes.txt"), []byte(volumes), 0o644); err != nil {
		t.Fatal(err)
	}

	steps := runWizard(t,
		filepath.Join(spkDir(t), "WIZARD_UIFILES", "upgrade_uifile.sh"),
		pkgvar, nil)
	defaults := assertWizardShape(t, steps)

	if defaults["wizard_port"] != "9998" {
		t.Errorf("prefill wizard_port = %q", defaults["wizard_port"])
	}
	if defaults["wizard_db_host"] != "127.0.0.1" {
		t.Errorf("prefill wizard_db_host = %q", defaults["wizard_db_host"])
	}
	if defaults["wizard_db_port"] != "3307" {
		t.Errorf("prefill wizard_db_port = %q", defaults["wizard_db_port"])
	}
	if defaults["wizard_db_name"] != "xbvr" {
		t.Errorf("prefill wizard_db_name = %q", defaults["wizard_db_name"])
	}
	if defaults["wizard_db_user"] != "xbvr" {
		t.Errorf("prefill wizard_db_user = %q", defaults["wizard_db_user"])
	}
	if defaults["wizard_db_pass"] != tricky {
		t.Errorf("prefill wizard_db_pass = %q, want tricky value intact", defaults["wizard_db_pass"])
	}
	if defaults["wizard_db_use"] != "true" {
		t.Errorf("prefill wizard_db_use = %q, want true for mysql URL", defaults["wizard_db_use"])
	}
	if defaults["wizard_video_subdirs"] != "/volume1/video,/volume2/more vr" {
		t.Errorf("prefill wizard_video_subdirs = %q", defaults["wizard_video_subdirs"])
	}

	// sqlite .env pre-fills the checkbox off.
	pkgvar2 := t.TempDir()
	if err := os.WriteFile(filepath.Join(pkgvar2, ".env"), []byte("DATABASE_URL=''\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	steps2 := runWizard(t,
		filepath.Join(spkDir(t), "WIZARD_UIFILES", "upgrade_uifile.sh"),
		pkgvar2, nil)
	defaults2 := assertWizardShape(t, steps2)
	if defaults2["wizard_db_use"] != "false" {
		t.Errorf("prefill wizard_db_use = %q, want false for sqlite", defaults2["wizard_db_use"])
	}
}

// runHook runs one service-setup hook with a controlled environment.
// Map entries override the base variables (no duplicate keys).
func runHook(t *testing.T, pkgvar, dest string, env map[string]string, hook string) error {
	t.Helper()
	cmd := exec.Command("sh", "-c", `. ./service-setup; `+hook)
	cmd.Dir = filepath.Join(spkDir(t), "scripts")
	base := map[string]string{
		"SYNOPKG_PKGVAR":  pkgvar,
		"SYNOPKG_PKGDEST": dest,
		"PATH":            "/usr/bin:/bin",
	}
	for k, v := range env {
		base[k] = v
	}
	for k, v := range base {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("%s output: %s", hook, out)
	}
	return err
}

// postinst with MariaDB chosen: creates the database idempotently via a
// fake mysql client, writes a .env with the URL (DSM port 3307 default),
// resolves the share into the volumes reminder, never stores the root
// password, and never overwrites an existing .env. (ffmpeg is untouched:
// XBVR self-downloads its static pair on first run.)
func TestServicePostinstWritesEnv(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}
	pkgvar := t.TempDir()
	dest := t.TempDir()

	fakebin := t.TempDir()

	// Fake mysql client capturing args + stdin.
	capture := filepath.Join(t.TempDir(), "mysql.log")
	fakeMysql := filepath.Join(fakebin, "mysql")
	mysqlStub := "#!/bin/sh\necho \"ARGS: $@\" >> \"" + capture + "\"\ncat >> \"" + capture + "\"\n"
	if err := os.WriteFile(fakeMysql, []byte(mysqlStub), 0o755); err != nil {
		t.Fatal(err)
	}

	// Fake /volume tree for share resolution.
	volroot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(volroot, "vol1", "porn", "vr"), 0o755); err != nil {
		t.Fatal(err)
	}

	wizard := map[string]string{
		"wizard_port":          "9999",
		"wizard_video_share":   "porn",
		"wizard_video_subdirs": "vr",
		"wizard_db_use":        "true",
		"wizard_db_host":       "127.0.0.1",
		"wizard_db_port":       "",
		"wizard_db_name":       "xbvr",
		"wizard_db_user":       "xbvr",
		"wizard_db_pass":       "p@ss #with 'quotes'",
		"wizard_db_root_pass":  "r00t!",
		"MYSQL_CLIENT":         fakeMysql,
		"SYNOPKG_VOLUME_GLOB":  filepath.Join(volroot, "vol*"),
	}
	cmd := exec.Command("sh", "-c", `. ./service-setup; service_postinst`)
	cmd.Dir = filepath.Join(spkDir(t), "scripts")
	cmd.Env = []string{
		"SYNOPKG_PKGVAR=" + pkgvar, "SYNOPKG_PKGDEST=" + dest,
		"PATH=" + fakebin + ":/usr/bin:/bin",
	}
	for k, v := range wizard {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("postinst failed: %v\n%s", err, out)
	}

	sql, err := os.ReadFile(capture)
	if err != nil {
		t.Fatalf("mysql client never ran: %v", err)
	}
	for _, want := range []string{
		"CREATE DATABASE IF NOT EXISTS `xbvr`",
		"CREATE USER IF NOT EXISTS 'xbvr'@'%'",
		"GRANT ALL PRIVILEGES ON `xbvr`.*",
		"-uroot", "-pr00t!",
	} {
		if !strings.Contains(string(sql), want) {
			t.Errorf("mysql call lacks %q:\n%s", want, sql)
		}
	}

	raw, err := os.ReadFile(filepath.Join(pkgvar, ".env"))
	if err != nil {
		t.Fatalf(".env not written: %v", err)
	}
	body := string(raw)
	if !strings.Contains(body, "DATABASE_URL='mysql://xbvr:") {
		t.Errorf(".env lacks mysql DATABASE_URL:\n%s", body)
	}
	if !strings.Contains(body, "@127.0.0.1:3307/xbvr'") {
		t.Errorf(".env lacks DSM MariaDB 3307 default:\n%s", body)
	}
	if !strings.Contains(body, "XBVR_APPDIR='"+pkgvar+"'") {
		t.Errorf(".env lacks XBVR_APPDIR:\n%s", body)
	}
	if strings.Contains(body, "r00t!") {
		t.Errorf(".env must never store the root password:\n%s", body)
	}
	st, _ := os.Stat(filepath.Join(pkgvar, ".env"))
	if st.Mode().Perm() != 0o600 {
		t.Errorf(".env mode = %o, want 600", st.Mode().Perm())
	}

	rem, err := os.ReadFile(filepath.Join(pkgvar, "volumes.txt"))
	if err != nil {
		t.Fatalf("volumes.txt not written: %v", err)
	}
	if !strings.Contains(string(rem), filepath.Join(volroot, "vol1", "porn", "vr")+"\n") {
		t.Errorf("volumes.txt lacks resolved share path:\n%s", rem)
	}

	// Second run keeps the user's file.
	sentinel := []byte("XBVR_WEB_PORT='1111'\n")
	if err := os.WriteFile(filepath.Join(pkgvar, ".env"), sentinel, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := runHook(t, pkgvar, dest, wizard, "service_postinst"); err != nil {
		t.Fatalf("second postinst failed: %v", err)
	}
	kept, _ := os.ReadFile(filepath.Join(pkgvar, ".env"))
	if string(kept) != string(sentinel) {
		t.Errorf("postinst overwrote existing .env:\n%s", kept)
	}
}

// MariaDB chosen but unreachable (or no client): loud failure, no .env.
func TestEnsureDbFailsLoudly(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}
	pkgvar := t.TempDir()
	// Client that always fails.
	fakebin := t.TempDir()
	failMysql := filepath.Join(fakebin, "mysql")
	if err := os.WriteFile(failMysql, []byte("#!/bin/sh\nexit 3\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	err := runHook(t, pkgvar, t.TempDir(), map[string]string{
		"wizard_db_use":  "true",
		"wizard_db_host": "127.0.0.1",
		"MYSQL_CLIENT":   failMysql,
		"PATH":           fakebin + ":/usr/bin:/bin",
	}, "service_postinst")
	if err == nil {
		t.Error("postinst with unreachable MariaDB succeeded; want failure")
	}
	if _, statErr := os.Stat(filepath.Join(pkgvar, ".env")); !os.IsNotExist(statErr) {
		t.Error(".env written despite failed DB setup; want nothing")
	}
}

// MariaDB root refused over TCP (passwordless wizard default) but reachable
// over the local socket: postinst must use the socket instead of failing.
// This is the DS1815+ failure on syno-v0.4.40-3: DSM injects the MariaDB-on
// wizard defaults into CLI installs, TCP root demands a password, socket
// root (unix_socket plugin) does not.
func TestEnsureDbSocketFallback(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}
	pkgvar := t.TempDir()
	dest := t.TempDir()

	// Real unix socket so `[ -S ]` passes.
	sockDir := t.TempDir()
	sockPath := filepath.Join(sockDir, "mysqld.sock")
	l, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Skipf("cannot create unix socket: %v", err)
	}
	defer l.Close()

	// Fake mysql: TCP (-h) always refused, socket (-S) works and logs.
	capture := filepath.Join(t.TempDir(), "mysql.log")
	fakebin := t.TempDir()
	stub := "#!/bin/sh\nfor a in \"$@\"; do case \"$a\" in -h*) exit 1;; esac; done\n" +
		"echo \"ARGS: $@\" >> \"" + capture + "\"\ncat >> \"" + capture + "\"\n"
	if err := os.WriteFile(filepath.Join(fakebin, "mysql"), []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := runHook(t, pkgvar, dest, map[string]string{
		"wizard_db_use":     "true",
		"wizard_db_host":    "127.0.0.1",
		"wizard_db_pass":    "",
		"MYSQL_CLIENT":      filepath.Join(fakebin, "mysql"),
		"MYSQL_SOCKET_GLOB": filepath.Join(sockDir, "*.sock"),
		"PATH":              fakebin + ":/usr/bin:/bin",
	}, "service_postinst"); err != nil {
		t.Fatalf("postinst via socket failed: %v", err)
	}
	sql, err := os.ReadFile(capture)
	if err != nil {
		t.Fatalf("mysql client never ran over socket: %v", err)
	}
	for _, want := range []string{
		"-S" + sockPath,
		"CREATE DATABASE IF NOT EXISTS `xbvr`",
		"CREATE USER IF NOT EXISTS 'xbvr'@'%'",
	} {
		if !strings.Contains(string(sql), want) {
			t.Errorf("mysql socket call lacks %q:\n%s", want, sql)
		}
	}
	raw, err := os.ReadFile(filepath.Join(pkgvar, ".env"))
	if err != nil {
		t.Fatalf(".env not written: %v", err)
	}
	if !strings.Contains(string(raw), "DATABASE_URL='mysql://xbvr:") {
		t.Errorf(".env lacks mysql DATABASE_URL:\n%s", raw)
	}
}

// postinst hands its files to the var-dir owner: install scripts run as
// root while the daemon runs as the service user, and an unreadable .env
// aborts prestart (DS1815+ "Failed to run the package service").
func TestPostinstClaimsVarFiles(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}
	pkgvar := t.TempDir()
	dest := t.TempDir()

	// Pre-existing bin dir (upgrade scenario): claimed, not managed.
	if err := os.MkdirAll(filepath.Join(pkgvar, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Fake chown capturing its args (proves wiring without privileges).
	capture := filepath.Join(t.TempDir(), "chown.log")
	fakebin := t.TempDir()
	stub := "#!/bin/sh\necho \"ARGS: $@\" >> \"" + capture + "\"\n"
	if err := os.WriteFile(filepath.Join(fakebin, "chown"), []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := runHook(t, pkgvar, dest, map[string]string{
		"wizard_db_use": "false",
		"PATH":          fakebin + ":/usr/bin:/bin",
	}, "service_postinst"); err != nil {
		t.Fatalf("postinst failed: %v", err)
	}
	raw, err := os.ReadFile(capture)
	if err != nil {
		t.Fatalf("chown never ran: %v", err)
	}
	for _, want := range []string{
		filepath.Join(pkgvar, ".env"),
		filepath.Join(pkgvar, "bin"),
	} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("chown missing %q:\n%s", want, raw)
		}
	}
	if strings.Contains(string(raw), "volumes.txt") {
		t.Errorf("chown should skip absent volumes.txt:\n%s", raw)
	}
	st, _ := os.Stat(filepath.Join(pkgvar, ".env"))
	if st.Mode().Perm() != 0o600 {
		t.Errorf(".env mode = %o, want 600", st.Mode().Perm())
	}
}

// postinst grants the service user DSM read access on the wizard video
// dirs (the exact File Station "Read" ACE), and skips dirs that already
// grant it anything. synoacltool is faked; the mask asserted is the one
// DSM itself writes.
func TestGrantVideoAccess(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}
	pkgvar := t.TempDir()
	dest := t.TempDir()

	volroot := t.TempDir()
	target := filepath.Join(volroot, "vol1", "porn", "vr")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}

	capture := filepath.Join(t.TempDir(), "acl.log")
	fakebin := t.TempDir()
	stub := "#!/bin/sh\necho \"ARGS: $@\" >> \"" + capture + "\"\n" +
		"if [ \"$1\" = \"-get\" ]; then\n" +
		"  if [ -n \"$FAKE_ACE\" ]; then echo \"$FAKE_ACE\"; fi\n" +
		"  exit 0\nfi\nexit 0\n"
	if err := os.WriteFile(filepath.Join(fakebin, "synoacltool"), []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}

	base := map[string]string{
		"wizard_video_share":   "porn",
		"wizard_video_subdirs": "vr",
		"wizard_db_use":        "false",
		"SYNOPKG_VOLUME_GLOB":  filepath.Join(volroot, "vol*"),
		"PATH":                 fakebin + ":/usr/bin:/bin",
	}
	if err := runHook(t, pkgvar, dest, base, "service_postinst"); err != nil {
		t.Fatalf("postinst failed: %v", err)
	}
	raw, err := os.ReadFile(capture)
	if err != nil {
		t.Fatalf("synoacltool never ran: %v", err)
	}
	if !strings.Contains(string(raw), "-add "+target+" ") {
		t.Errorf("no -add for %q:\n%s", target, raw)
	}
	if !strings.Contains(string(raw), "allow:r-x---a-R-c--:fd--") {
		t.Errorf("no File Station Read ACE:\n%s", raw)
	}

	// A dir that already grants the owner is left alone.
	me, err := user.Current()
	if err != nil {
		t.Skipf("no current user: %v", err)
	}
	if err := os.Remove(capture); err != nil {
		t.Fatal(err)
	}
	present := map[string]string{
		"FAKE_ACE": "user:" + me.Username + ":allow:rwxpdDaARWc--:fd--",
	}
	for k, v := range base {
		present[k] = v
	}
	pkgvar2 := t.TempDir()
	if err := runHook(t, pkgvar2, dest, present, "service_postinst"); err != nil {
		t.Fatalf("postinst failed: %v", err)
	}
	raw2, err := os.ReadFile(capture)
	if err != nil {
		t.Fatalf("synoacltool never ran: %v", err)
	}
	if strings.Contains(string(raw2), "-add ") {
		t.Errorf("-add ran despite existing ACE:\n%s", raw2)
	}
}

// The share box tolerates a full path and padding whitespace instead of
// failing the install (a GUI upgrade submitted "/volume2/porn").
func TestVolumeDirsTolerance(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}
	volroot := t.TempDir()
	virtual := filepath.Join(volroot, "vol1", "porn", "virtual")
	if err := os.MkdirAll(virtual, 0o755); err != nil {
		t.Fatal(err)
	}
	absdir := t.TempDir()
	runDirs := func(t *testing.T, env map[string]string) (string, error) {
		t.Helper()
		cmd := exec.Command("sh", "-c", `. ./service-setup; volume_dirs`)
		cmd.Dir = filepath.Join(spkDir(t), "scripts")
		cmd.Env = []string{
			"SYNOPKG_PKGVAR=" + t.TempDir(), "SYNOPKG_PKGDEST=" + t.TempDir(),
			"PATH=/usr/bin:/bin",
			"SYNOPKG_VOLUME_GLOB=" + filepath.Join(volroot, "vol*"),
		}
		for k, v := range env {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
		out, err := cmd.Output()
		return strings.TrimSpace(string(out)), err
	}
	if got, err := runDirs(t, map[string]string{
		"wizard_video_share": "porn ", "wizard_video_subdirs": "virtual",
	}); err != nil || got != virtual {
		t.Errorf("padded share = %q, %v; want %q", got, err, virtual)
	}
	if got, err := runDirs(t, map[string]string{
		"wizard_video_share": absdir,
	}); err != nil || got != absdir {
		t.Errorf("absolute share = %q, %v; want %q", got, err, absdir)
	}
	if got, err := runDirs(t, map[string]string{
		"wizard_video_share": "/nonexistent-share",
	}); err == nil {
		t.Errorf("missing absolute share = %q, nil error; want failure", got)
	}
}

// Explicit sqlite opt-out keeps the fallback and skips MariaDB entirely.
func TestExplicitSqliteSkipsMariaDB(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}
	pkgvar := t.TempDir()
	if err := runHook(t, pkgvar, t.TempDir(), map[string]string{
		"wizard_port":   "9999",
		"wizard_db_use": "false",
	}, "service_postinst"); err != nil {
		t.Fatalf("postinst failed: %v", err)
	}
	raw, _ := os.ReadFile(filepath.Join(pkgvar, ".env"))
	if !strings.Contains(string(raw), "DATABASE_URL=''") {
		t.Errorf("sqlite opt-out should leave DATABASE_URL empty:\n%s", raw)
	}
}
