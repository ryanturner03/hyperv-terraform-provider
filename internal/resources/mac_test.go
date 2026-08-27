package resources

import "testing"

func TestNormalizeMAC(t *testing.T) {
	cases := map[string]string{
		"00155DC0B668":      "00:15:5D:C0:B6:68", // Hyper-V read-back form
		"00:15:5D:C0:B6:68": "00:15:5D:C0:B6:68", // already canonical
		"00:15:5d:c0:b6:68": "00:15:5D:C0:B6:68", // lower-case
		"00-15-5D-C0-B6-68": "00:15:5D:C0:B6:68", // hyphen form
		"":                  "",                  // unset
		"not-a-mac":         "not-a-mac",         // left alone
		"00155DC0B6":        "00155DC0B6",        // too short, left alone
	}
	for in, want := range cases {
		if got := normalizeMAC(in); got != want {
			t.Errorf("normalizeMAC(%q) = %q, want %q", in, got, want)
		}
	}
}
