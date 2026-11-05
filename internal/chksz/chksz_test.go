package chksz

import "testing"

// A stream's format comes from its file, whose URL carries the signature
// in its query.
func TestTypeOf(t *testing.T) {
	cases := map[string]string{
		"https://m801.music.126.net/2026/a/4b/c.flac?vuutv=abc+def=": "flac",
		"https://m701.music.126.net/2026/a/4b/c.mp3?vuutv=abc":       "mp3",
		"https://m701.music.126.net/2026/a/4b/c":                     "mp3",
	}
	for u, want := range cases {
		if got := typeOf(u); got != want {
			t.Errorf("%s: %s, want %s", u, got, want)
		}
	}
}
