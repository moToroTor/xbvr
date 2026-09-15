package tasks

import (
	"reflect"
	"testing"
)

func TestParseJavCodes(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "one per line",
			input: "3DSVR-0520\nABC-123\n",
			want:  []string{"3DSVR-0520", "ABC-123"},
		},
		{
			name:  "comma separated with spaces",
			input: "3DSVR-0520, ABC-123 ,DEF-456",
			want:  []string{"3DSVR-0520", "ABC-123", "DEF-456"},
		},
		{
			name:  "blank lines dropped",
			input: "\n3DSVR-0520\n\n\nABC-123\n",
			want:  []string{"3DSVR-0520", "ABC-123"},
		},
		{
			name:  "duplicates dropped, order kept",
			input: "ABC-123\n3DSVR-0520\nABC-123\n",
			want:  []string{"ABC-123", "3DSVR-0520"},
		},
		{
			name:  "empty input yields nothing",
			input: "  \n, ,\n",
			want:  nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ParseJavCodes(tc.input); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("ParseJavCodes(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
