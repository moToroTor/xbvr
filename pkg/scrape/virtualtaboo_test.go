package scrape

import "testing"

func TestVirtualTabooSiteIDFromHref(t *testing.T) {
	cases := []struct {
		name string
		href string
		want string
	}{
		{"signup link with vid", "https://join.virtualtaboo.com/signup/signup.php?vid=1520", "1520"},
		{"signup link with extra params", "https://join.virtualtaboo.com/signup/signup.php?nats=MTYuNi4xLjEuNi4wLjAuMC4w&vid=1520", "1520"},
		{"relative href with vid", "/signup/signup.php?vid=73", "73"},
		{"no vid parameter", "https://virtualtaboo.com/videos/big-deal-with-stepdaddy-vt73k", ""},
		{"empty href", "", ""},
		{"vid without digits", "https://join.virtualtaboo.com/signup/signup.php?vid=", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := virtualTabooSiteIDFromHref(c.href); got != c.want {
				t.Errorf("virtualTabooSiteIDFromHref(%q) = %q, want %q", c.href, got, c.want)
			}
		})
	}
}
