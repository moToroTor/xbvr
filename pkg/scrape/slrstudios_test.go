package scrape

import (
	"testing"

	"github.com/tidwall/gjson"
)

func TestSlrScriptFlags(t *testing.T) {
	cases := []struct {
		name      string
		scripts   string
		wantHuman bool
		wantAi    bool
	}{
		{
			name:      "human script only",
			scripts:   `[{"id":3484,"scriptAI":false}]`,
			wantHuman: true,
			wantAi:    false,
		},
		{
			name:      "ai script only",
			scripts:   `[{"id":3526,"scriptAI":true}]`,
			wantHuman: false,
			wantAi:    true,
		},
		{
			name:      "both human and ai scripts",
			scripts:   `[{"id":3484,"scriptAI":false},{"id":3526,"scriptAI":true}]`,
			wantHuman: true,
			wantAi:    true,
		},
		{
			name:      "empty array sets neither",
			scripts:   `[]`,
			wantHuman: false,
			wantAi:    false,
		},
		{
			name:      "entry without scriptAI key counts as human",
			scripts:   `[{"id":1}]`,
			wantHuman: true,
			wantAi:    false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			human, ai := slrScriptFlags(gjson.Parse(tc.scripts))
			if human != tc.wantHuman || ai != tc.wantAi {
				t.Errorf("slrScriptFlags(%s) = (%v, %v), want (%v, %v)",
					tc.scripts, human, ai, tc.wantHuman, tc.wantAi)
			}
		})
	}
}

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
