package scrape

import "testing"

func TestWankitnowMembersURL(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "public scene URL maps to members",
			in:   "https://wankitnowvr.com/videos/some-scene-12345",
			want: "https://members.wankitnowvr.com/m/videos/some-scene-12345",
		},
		{
			name: "already members URL is unchanged",
			in:   "https://members.wankitnowvr.com/m/videos/some-scene-12345",
			want: "https://members.wankitnowvr.com/m/videos/some-scene-12345",
		},
		{
			name: "other host passes through unchanged",
			in:   "https://zexyvr.com/videos/some-scene-12345",
			want: "https://zexyvr.com/videos/some-scene-12345",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := wankitnowMembersURL(tc.in); got != tc.want {
				t.Errorf("wankitnowMembersURL(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
