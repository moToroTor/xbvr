package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// DSM's Open button goes to http://host:adminport/adminurl. XBVR serves
// its UI at / with /api/* beside it, so any adminurl prefix would 404.
// Like Radarr/Jackett, declare only adminport so DSM opens the root.
// The ffmpeg6 dependency must stay: postinst symlinks its binaries.
func TestInfoOpensAtRoot(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(spkDir(t), "INFO"))
	if err != nil {
		t.Fatal(err)
	}
	fields := map[string]string{}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			t.Errorf("INFO line without =: %q", line)
			continue
		}
		fields[strings.TrimSpace(k)] = strings.Trim(strings.TrimSpace(v), `"`)
	}
	if fields["adminport"] != "9999" {
		t.Errorf("adminport = %q, want 9999", fields["adminport"])
	}
	if u, ok := fields["adminurl"]; ok && u != "" {
		t.Errorf("adminurl = %q, want absent (DSM would open a 404 path)", u)
	}
	if !strings.Contains(fields["install_dep_packages"], "ffmpeg6") {
		t.Errorf("install_dep_packages = %q, want ffmpeg6 dependency", fields["install_dep_packages"])
	}
	if fields["package"] != "xbvr" {
		t.Errorf("package = %q, want xbvr", fields["package"])
	}
}
