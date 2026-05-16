package update

import "testing"

func TestIsTickyRemote(t *testing.T) {
	cases := map[string]bool{
		"https://github.com/wingitman/ticky.git": true,
		"https://github.com/wingitman/ticky":     true,
		"git@github.com:wingitman/ticky.git":     true,
		"git@github.com:someone/ticky":           true,
		"https://github.com/wingitman/other.git": false,
	}
	for remote, want := range cases {
		if got := isTickyRemote(remote); got != want {
			t.Fatalf("isTickyRemote(%q) = %v, want %v", remote, got, want)
		}
	}
}
