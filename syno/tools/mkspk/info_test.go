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
// ffmpeg6 is deliberately NOT a dependency and postinst does no ffmpeg
// handling: XBVR self-downloads its static pair, and measured
// probe results show no ffmpeg6 advantage. A hard dep only blocks
// installs.
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
	if fields["install_dep_packages"] != "" {
		t.Errorf("install_dep_packages = %q, want empty (ffmpeg6 is opportunistic, not required)", fields["install_dep_packages"])
	}
	if fields["package"] != "xbvr" {
		t.Errorf("package = %q, want xbvr", fields["package"])
	}
}
