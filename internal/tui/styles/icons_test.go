package styles

import "testing"

func TestSupportsUnicodeAcrossLocales(t *testing.T) {
	for _, tc := range []struct {
		name   string
		env    map[string]string
		want   bool
		reason string
	}{
		{
			name:   "C.UTF-8 is a Unicode locale",
			env:    map[string]string{"LANG": "C.UTF-8"},
			want:   true,
			reason: "the default on most Linux distributions, container images and CI runners",
		},
		{name: "en_US.UTF-8", env: map[string]string{"LANG": "en_US.UTF-8"}, want: true},
		{name: "lowercase utf8 spelling", env: map[string]string{"LANG": "en_US.utf8"}, want: true},
		{name: "bare C is ASCII only", env: map[string]string{"LANG": "C"}, want: false},
		{name: "POSIX is ASCII only", env: map[string]string{"LANG": "POSIX"}, want: false},
		{name: "no locale set defaults to Unicode", env: nil, want: true},
		{name: "LC_ALL wins over LANG", env: map[string]string{"LC_ALL": "C", "LANG": "en_US.UTF-8"}, want: false},
		{name: "explicit ASCII override", env: map[string]string{"AUX_ASCII_ICONS": "1", "LANG": "en_US.UTF-8"}, want: false},
		{
			name:   "explicit Unicode override beats a C locale",
			env:    map[string]string{"AUX_UNICODE_ICONS": "1", "LANG": "C"},
			want:   true,
			reason: "lets tests and users pin rendering instead of inheriting the environment",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, k := range []string{"LC_ALL", "LC_CTYPE", "LANG", "AUX_ASCII_ICONS", "AUX_UNICODE_ICONS"} {
				t.Setenv(k, "")
			}
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			if got := SupportsUnicode(); got != tc.want {
				t.Errorf("SupportsUnicode() = %v, want %v. %s", got, tc.want, tc.reason)
			}
		})
	}
}
