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
