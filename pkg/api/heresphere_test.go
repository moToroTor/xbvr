package api

import (
	"testing"
)

// The per-file Heresphere endpoint omitted projection fields, so overrides
// never reached Heresphere on that path. Guards the mapping parity with the
// scene handler's switch.
func TestHeresphereProjection(t *testing.T) {
	cases := []struct {
		in                 string
		projection, stereo string
		fov                float64
		lens               string
	}{
		{"", "equirectangular", "sbs", 180, "Linear"},
		{"unknown-thing", "equirectangular", "sbs", 180, "Linear"},
		{"flat", "perspective", "mono", 180, "Linear"},
		{"180_mono", "equirectangular", "mono", 180, "Linear"},
		{"360_mono", "equirectangular360", "mono", 180, "Linear"},
		{"180_sbs", "equirectangular", "sbs", 180, "Linear"},
		{"360_tb", "equirectangular360", "tb", 180, "Linear"},
		{"mkx200", "fisheye", "sbs", 200, "MKX200"},
		{"mkx220", "fisheye", "sbs", 220, "MKX220"},
		{"vrca220", "fisheye", "sbs", 220, "VRCA220"},
		{"rf52", "fisheye", "sbs", 190, "Linear"},
		{"fisheye190", "fisheye", "sbs", 190, "Linear"},
		{"fisheye", "fisheye", "sbs", 180, "Linear"},
	}
	for _, c := range cases {
		projection, stereo, fov, lens := heresphereProjection(c.in)
		if projection != c.projection || stereo != c.stereo || fov != c.fov || lens != c.lens {
			t.Errorf("heresphereProjection(%q) = (%q, %q, %v, %q), want (%q, %q, %v, %q)",
				c.in, projection, stereo, fov, lens,
				c.projection, c.stereo, c.fov, c.lens)
		}
	}
}
