package scrape

import "testing"

func TestSlrLabelFallback(t *testing.T) {
	cases := []struct {
		name       string
		sceneID    string
		sceneLabel string
		want       string
	}{
		{
			name:       "emoticon label falls back to numeric id",
			sceneID:    "51115",
			sceneLabel: "cum-dripping-out-of-my-pussy💦-51115",
			want:       "51115",
		},
		{
			name:       "plain label needs no fallback",
			sceneID:    "51115",
			sceneLabel: "cum-dripping-out-of-my-pussy-51115",
			want:       "51115",
		},
		{
			name:       "label already numeric needs no retry",
			sceneID:    "51115",
			sceneLabel: "51115",
			want:       "",
		},
		{
			name:       "non-numeric id never retries",
			sceneID:    "abc-12",
			sceneLabel: "some-slug-abc-12",
			want:       "",
		},
		{
			name:       "empty id never retries",
			sceneID:    "",
			sceneLabel: "some-slug",
			want:       "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := slrLabelFallback(tc.sceneID, tc.sceneLabel); got != tc.want {
				t.Errorf("slrLabelFallback(%q, %q) = %q, want %q",
					tc.sceneID, tc.sceneLabel, got, tc.want)
			}
		})
	}
}
