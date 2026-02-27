package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"time"
)

const (
	defaultGitHubOwner = "tom-lisboa"
	defaultGitHubRepo  = "deepH"
)

func cmdUpdate(args []string) error {
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	owner := fs.String("owner", envOrDefault("DEEPH_GITHUB_OWNER", defaultGitHubOwner), "GitHub owner")
	repo := fs.String("repo", envOrDefault("DEEPH_GITHUB_REPO", defaultGitHubRepo), "GitHub repository")
	tag := fs.String("tag", "latest", "release tag (latest or vX.Y.Z)")
	checkOnly := fs.Bool("check", false, "only print target asset URL without installing")
	timeoutMS := fs.Int("timeout-ms", 60000, "download timeout in milliseconds")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(fs.Args()) != 0 {
		return errors.New("update does not accept positional arguments")
	}
	if strings.TrimSpace(*owner) == "" {
		return errors.New("--owner cannot be empty")
	}
	if strings.TrimSpace(*repo) == "" {
		return errors.New("--repo cannot be empty")
	}
	if *timeoutMS < 1000 {
		return errors.New("--timeout-ms must be >= 1000")
	}

	assetName, err := releaseAssetNameForPlatform(goruntime.GOOS, goruntime.GOARCH)
	if err != nil {
		return err
	}
	url := releaseAssetURL(strings.TrimSpace(*owner), strings.TrimSpace(*repo), strings.TrimSpace(*tag), assetName)

	if *checkOnly {
		fmt.Printf("update check:\n")
		fmt.Printf("  owner/repo: %s/%s\n", strings.TrimSpace(*owner), strings.TrimSpace(*repo))
		fmt.Printf("  target: %s/%s\n", goruntime.GOOS, goruntime.GOARCH)
		fmt.Printf("  asset: %s\n", assetName)
		fmt.Printf("  url: %s\n", url)
		return nil
	}

	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable path: %w", err)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		// keep original path when symlink resolution is not available
		execPath, _ = os.Executable()
	}
	execDir := filepath.Dir(execPath)

	tmpPath := filepath.Join(execDir, "deeph.update.tmp")
	if goruntime.GOOS == "windows" {
		tmpPath = filepath.Join(execDir, "deeph.new.exe")
	}
	if err := downloadFileWithTimeout(url, tmpPath, time.Duration(*timeoutMS)*time.Millisecond); err != nil {
		return err
	}
	if goruntime.GOOS != "windows" {
		if err := os.Chmod(tmpPath, 0o755); err != nil {
			_ = os.Remove(tmpPath)
			return fmt.Errorf("chmod updated binary: %w", err)
		}
		if err := os.Rename(tmpPath, execPath); err != nil {
			_ = os.Remove(tmpPath)
			return fmt.Errorf("replace executable: %w", err)
		}
		fmt.Printf("Updated deeph successfully.\n")
		fmt.Printf("  binary: %s\n", execPath)
		fmt.Printf("  source: %s\n", url)
		return nil
	}

	fmt.Printf("Downloaded update to %s\n", tmpPath)
	fmt.Println("Windows replacement steps:")
	fmt.Println("  1. Close all deeph terminals")
	fmt.Printf("  2. Rename %s to deeph.old.exe\n", filepath.Base(execPath))
	fmt.Printf("  3. Rename %s to %s\n", filepath.Base(tmpPath), filepath.Base(execPath))
	return nil
}

func releaseAssetNameForPlatform(goos, goarch string) (string, error) {
	switch goos {
	case "darwin":
		switch goarch {
		case "arm64":
			return "deeph-darwin-arm64", nil
		case "amd64":
			return "deeph-darwin-amd64", nil
		}
	case "linux":
		switch goarch {
		case "arm64":
			return "deeph-linux-arm64", nil
		case "amd64":
			return "deeph-linux-amd64", nil
		}
	case "windows":
		switch goarch {
		case "arm64":
			return "deeph-windows-arm64.exe", nil
		case "amd64":
			return "deeph-windows-amd64.exe", nil
		}
	}
	return "", fmt.Errorf("unsupported platform for update: %s/%s", goos, goarch)
}

func releaseAssetURL(owner, repo, tag, asset string) string {
	normalizedTag := strings.TrimSpace(tag)
	if normalizedTag == "" || strings.EqualFold(normalizedTag, "latest") {
		return fmt.Sprintf("https://github.com/%s/%s/releases/latest/download/%s", owner, repo, asset)
	}
	return fmt.Sprintf("https://github.com/%s/%s/releases/download/%s/%s", owner, repo, normalizedTag, asset)
}

func downloadFileWithTimeout(url, outPath string, timeout time.Duration) error {
	tmpDir := filepath.Dir(outPath)
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}
	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create update request: %w", err)
	}
	req.Header.Set("User-Agent", "deeph-cli-updater")
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download update: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("download update failed status=%d body=%q", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	f, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("create output file: %w", err)
	}
	defer func() {
		_ = f.Close()
	}()
	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("write output file: %w", err)
	}
	return nil
}

func envOrDefault(key, fallback string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	return v
}
