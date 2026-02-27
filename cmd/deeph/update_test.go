package main

import "testing"

func TestReleaseAssetNameForPlatform(t *testing.T) {
	t.Parallel()

	cases := []struct {
		goos   string
		goarch string
		want   string
		ok     bool
	}{
		{goos: "darwin", goarch: "arm64", want: "deeph-darwin-arm64", ok: true},
		{goos: "darwin", goarch: "amd64", want: "deeph-darwin-amd64", ok: true},
		{goos: "linux", goarch: "arm64", want: "deeph-linux-arm64", ok: true},
		{goos: "linux", goarch: "amd64", want: "deeph-linux-amd64", ok: true},
		{goos: "windows", goarch: "arm64", want: "deeph-windows-arm64.exe", ok: true},
		{goos: "windows", goarch: "amd64", want: "deeph-windows-amd64.exe", ok: true},
		{goos: "freebsd", goarch: "amd64", ok: false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.goos+"-"+tc.goarch, func(t *testing.T) {
			t.Parallel()
			got, err := releaseAssetNameForPlatform(tc.goos, tc.goarch)
			if tc.ok && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !tc.ok && err == nil {
				t.Fatalf("expected error, got asset %q", got)
			}
			if tc.ok && got != tc.want {
				t.Fatalf("asset mismatch: got %q want %q", got, tc.want)
			}
		})
	}
}

func TestReleaseAssetURL(t *testing.T) {
	t.Parallel()

	u := releaseAssetURL("acme", "deeph", "latest", "deeph-linux-amd64")
	if u != "https://github.com/acme/deeph/releases/latest/download/deeph-linux-amd64" {
		t.Fatalf("unexpected latest URL: %s", u)
	}

	u = releaseAssetURL("acme", "deeph", "v1.2.3", "deeph-linux-amd64")
	if u != "https://github.com/acme/deeph/releases/download/v1.2.3/deeph-linux-amd64" {
		t.Fatalf("unexpected tag URL: %s", u)
	}
}
