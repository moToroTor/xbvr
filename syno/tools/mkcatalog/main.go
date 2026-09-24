// Command mkcatalog builds the static Package Center catalog JSON from
// mkspk manifest fragments.
//
// Usage:
//
//	go run ./tools/mkcatalog --manifests 'dist/*.manifest.json' \
//	    --assets https://motorotor.github.io/xbvr/dl \
//	    --out catalog.json
//
// The output matches spkrepo's catalog entry shape (package, version,
// dname, desc, link, thumbnail, qinst/qupgrade/qstart, md5, size, …)
// so DSM Package Center accepts it from a static host like GitHub Pages.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	pattern := flag.String("manifests", "dist/*.manifest.json", "glob of mkspk manifest fragments")
	assets := flag.String("assets", "", "base URL hosting the .spk and icon files")
	out := flag.String("out", "catalog.json", "output catalog path")
	flag.Parse()
	if *assets == "" {
		fmt.Fprintln(os.Stderr, "assets base URL is required")
		os.Exit(1)
	}
	if err := run(*pattern, strings.TrimRight(*assets, "/"), *out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(pattern, assets, out string) error {
	files, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}
	sort.Strings(files)
	pkgs := []map[string]any{}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			return fmt.Errorf("%s: %w", f, err)
		}
		filename, _ := m["filename"].(string)
		icon72 := strings.TrimSuffix(filename, ".spk") + ".icon_72.png"
		icon256 := strings.TrimSuffix(filename, ".spk") + ".icon_256.png"
		pkgs = append(pkgs, map[string]any{
			"package":          "xbvr",
			"arch":             m["arch"],
			"version":          m["version"],
			"dname":            "XBVR",
			"desc":             "XBVR media server: local VR library with metadata scraping.",
			"link":             assets + "/" + filename,
			"thumbnail":        []string{assets + "/" + icon72, assets + "/" + icon256},
			"thumbnail_retina": []string{assets + "/" + icon256, assets + "/" + icon256},
			"snapshot":         []string{},
			"qinst":            true,
			"qupgrade":         true,
			"qstart":           true,
			"startable":        "yes",
			"md5":              m["md5"],
			"size":             m["size"],
			"changelog":        "See https://github.com/moToroTor/xbvr/releases",
			"distributor":      "moToroTor",
			"distributor_url":  "https://github.com/moToroTor/xbvr",
			"maintainer":       "moToroTor",
			"maintainer_url":   "https://github.com/moToroTor/xbvr",
		})
	}
	doc, _ := json.MarshalIndent(map[string]any{"packages": pkgs}, "", "  ")
	if err := os.WriteFile(out, append(doc, '\n'), 0o644); err != nil {
		return err
	}
	fmt.Printf("wrote %s with %d package(s)\n", out, len(pkgs))
	return nil
}
