// Command mkspk assembles a DSM 7 .spk from a built xbvr binary and
// the spk/ skeleton (INFO, scripts, conf, icons, wizards).
//
// Usage:
//
//	go run ./tools/mkspk --binary dist/xbvr-linux-amd64 --arch avoton \
//	    --version 0.4.40 --rev 1 --fw 7.1-42661 --spk-dir spk --out dist
//
// Output: dist/xbvr_{arch}-{fwshort}_{version}-{rev}.spk plus a
// <name>.manifest.json fragment (filename, arch, version, size, md5)
// for the catalog generator.
//
// Conventions learned from the Wankarr packaging (all load-bearing):
//   - outer .spk is plain tar; only package.tgz is gzipped (DSM rejects
//     a gzipped outer as "Invalid file format");
//   - six per-action stub scripts beside installer (error 261 otherwise);
//   - everything runs as the service user (DSM blocks root privileges);
//   - no adminurl in INFO (DSM would open a 404 path; UI serves at /).
package main

import (
	"archive/tar"
	"compress/gzip"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var fixedTime = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func main() {
	binary := flag.String("binary", "", "built xbvr binary (GOOS/GOARCH matched, CGO against glibc <= 2.26)")
	arch := flag.String("arch", "", "DSM arch code, e.g. avoton, aarch64")
	version := flag.String("version", "", "package version, e.g. 0.4.40")
	rev := flag.String("rev", "1", "spk revision")
	fw := flag.String("fw", "7.1-42661", "firmware string")
	spkDir := flag.String("spk-dir", "spk", "skeleton directory")
	icon := flag.String("icon", "", "256x256 source PNG (upstream ui/public/icons/xbvr-256.png)")
	out := flag.String("out", "dist", "output directory")
	flag.Parse()
	if *binary == "" || *arch == "" || *version == "" {
		fmt.Fprintln(os.Stderr, "binary, arch and version are required")
		os.Exit(1)
	}
	if err := run(*binary, *arch, *version, *rev, *fw, *spkDir, *icon, *out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(binary, arch, version, rev, fw, spkDir, iconSrc, out string) error {
	stage, err := os.MkdirTemp("", "xbvr-spk")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)

	read := func(rel string) []byte {
		b, err := os.ReadFile(filepath.Join(spkDir, rel))
		if err != nil {
			fmt.Fprintf(os.Stderr, "missing %s: %v\n", rel, err)
			os.Exit(1)
		}
		return b
	}

	// package.tgz payload: the server binary plus the .env template.
	pkgFiles := []tarFile{
		{Name: "bin/xbvr", Mode: 0o755, Data: mustRead(binary)},
		{Name: "share/xbvr/env-example", Mode: 0o644, Data: read("share/xbvr/env-example")},
	}
	pkgTgz := filepath.Join(stage, "package.tgz")
	if err := writeTar(pkgTgz, pkgFiles, true); err != nil {
		return err
	}

	fwShort := fw
	if i := strings.Index(fw, "-"); i >= 0 {
		fwShort = fw[:i]
	}
	info := strings.NewReplacer(
		"__VERSION__", version+"-"+rev,
		"__ARCH__", arch,
	).Replace(string(read("INFO")))

	iconDir := filepath.Join(stage, "icons")
	if err := os.MkdirAll(iconDir, 0o755); err != nil {
		return err
	}
	if err := makeIcons(iconSrc, iconDir); err != nil {
		return fmt.Errorf("icons: %w", err)
	}

	spkName := fmt.Sprintf("xbvr_%s-%s_%s-%s.spk", arch, fwShort, version, rev)
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	spkPath := filepath.Join(out, spkName)
	files := []tarFile{
		{Name: "INFO", Mode: 0o644, Data: []byte(info)},
		{Name: "scripts", Mode: 0o755, Dir: true},
		{Name: "conf", Mode: 0o755, Dir: true},
		{Name: "WIZARD_UIFILES", Mode: 0o755, Dir: true},
		{Name: "PACKAGE_ICON.PNG", Mode: 0o644, Data: mustRead(filepath.Join(iconDir, "PACKAGE_ICON.PNG"))},
		{Name: "PACKAGE_ICON_256.PNG", Mode: 0o644, Data: mustRead(filepath.Join(iconDir, "PACKAGE_ICON_256.PNG"))},
		{Name: "package.tgz", Mode: 0o644, Data: mustRead(pkgTgz)},
		{Name: "scripts/installer", Mode: 0o755, Data: read("scripts/installer")},
		{Name: "scripts/start-stop-status", Mode: 0o755, Data: read("scripts/start-stop-status")},
		{Name: "scripts/service-setup", Mode: 0o644, Data: read("scripts/service-setup")},
		{Name: "conf/privilege", Mode: 0o644, Data: read("conf/privilege")},
		{Name: "WIZARD_UIFILES/install_uifile.sh", Mode: 0o755, Data: read("WIZARD_UIFILES/install_uifile.sh")},
		{Name: "WIZARD_UIFILES/upgrade_uifile.sh", Mode: 0o755, Data: read("WIZARD_UIFILES/upgrade_uifile.sh")},
	}
	files = append(files, dsmActionScripts(read)...)
	if err := writeTar(spkPath, files, false); err != nil {
		return err
	}

	sum, size, err := digest(spkPath)
	if err != nil {
		return err
	}
	manifest, _ := json.MarshalIndent(map[string]any{
		"filename": spkName, "arch": arch, "fw": fw,
		"version": version + "-" + rev, "size": size, "md5": sum,
	}, "", "  ")
	manifestPath := filepath.Join(out, strings.TrimSuffix(spkName, ".spk")+".manifest.json")
	if err := os.WriteFile(manifestPath, append(manifest, '\n'), 0o644); err != nil {
		return err
	}
	fmt.Printf("wrote %s (%d bytes, md5 %s)\n", spkPath, size, sum)
	return nil
}

type tarFile struct {
	Name string
	Mode int64
	Data []byte
	Dir  bool
}

// dsmActionScripts returns the per-action entry-point scripts DSM requires
// alongside the installer dispatcher.
func dsmActionScripts(read func(string) []byte) []tarFile {
	var out []tarFile
	for _, a := range []string{
		"preinst", "postinst", "preuninst", "postuninst", "preupgrade", "postupgrade",
	} {
		out = append(out, tarFile{Name: "scripts/" + a, Mode: 0o755, Data: read("scripts/" + a)})
	}
	return out
}

func writeTar(path string, files []tarFile, gz bool) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	var tw *tar.Writer
	var gw *gzip.Writer
	if gz {
		gw = gzip.NewWriter(f)
		defer gw.Close()
		tw = tar.NewWriter(gw)
	} else {
		tw = tar.NewWriter(f)
	}
	defer tw.Close()
	for _, tf := range files {
		hdr := &tar.Header{
			Name: tf.Name, Mode: tf.Mode, Size: int64(len(tf.Data)),
			ModTime: fixedTime, Format: tar.FormatUSTAR,
			Uid: 0, Gid: 0, Uname: "root", Gname: "root",
		}
		if tf.Dir {
			hdr.Typeflag = tar.TypeDir
			hdr.Name += "/"
			hdr.Size = 0
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if !tf.Dir {
			if _, err := tw.Write(tf.Data); err != nil {
				return err
			}
		}
	}
	return nil
}

// makeIcons produces PACKAGE_ICON.PNG (72x72) and PACKAGE_ICON_256.PNG
// from the upstream 256px art (stdlib-only downscale), or a placeholder
// X when no source is given so layout tests still run.
func makeIcons(src, dir string) error {
	if src != "" {
		f, err := os.Open(src)
		if err != nil {
			return err
		}
		defer f.Close()
		img, _, err := image.Decode(f)
		if err != nil {
			return err
		}
		for _, s := range []struct {
			size int
			name string
		}{{72, "PACKAGE_ICON.PNG"}, {256, "PACKAGE_ICON_256.PNG"}} {
			out, err := os.Create(filepath.Join(dir, s.name))
			if err != nil {
				return err
			}
			err = png.Encode(out, nearest(img, s.size))
			out.Close()
			if err != nil {
				return err
			}
		}
		return nil
	}
	return genIcons(dir)
}

func nearest(img image.Image, size int) image.Image {
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	b := img.Bounds()
	sx, sy := float64(b.Dx())/float64(size), float64(b.Dy())/float64(size)
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dst.Set(x, y, img.At(b.Min.X+int(float64(x)*sx), b.Min.Y+int(float64(y)*sy)))
		}
	}
	return dst
}

// genIcons draws placeholder package icons (white X on dark blue).
func genIcons(dir string) error {
	draw := func(size int) *image.RGBA {
		img := image.NewRGBA(image.Rect(0, 0, size, size))
		bg := color.RGBA{0x1a, 0x1a, 0x2e, 0xff}
		fg := color.RGBA{0xff, 0xff, 0xff, 0xff}
		for y := 0; y < size; y++ {
			for x := 0; x < size; x++ {
				img.Set(x, y, bg)
			}
		}
		w := size / 12
		if w < 2 {
			w = 2
		}
		m := size / 5
		for i := m; i < size-m; i++ {
			for o := -w / 2; o <= w/2; o++ {
				img.Set(i, i+o, fg)
				img.Set(i, size-1-i+o, fg)
			}
		}
		return img
	}
	for _, s := range []struct {
		size int
		name string
	}{{72, "PACKAGE_ICON.PNG"}, {256, "PACKAGE_ICON_256.PNG"}} {
		f, err := os.Create(filepath.Join(dir, s.name))
		if err != nil {
			return err
		}
		if err := png.Encode(f, draw(s.size)); err != nil {
			f.Close()
			return err
		}
		f.Close()
	}
	return nil
}

func digest(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := md5.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

func mustRead(path string) []byte {
	b, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read %s: %v\n", path, err)
		os.Exit(1)
	}
	return b
}
