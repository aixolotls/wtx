package cmd

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	updateRepoPath         = "aixolotls/wtx"
	updateRepoGitURL       = "https://github.com/aixolotls/wtx.git"
	defaultUpdateInterval  = 10 * time.Minute
	startupUpdateTimeout   = 3 * time.Second
	resolveUpdateTimeout   = 8 * time.Second
	installUpdateTimeout   = 2 * time.Minute
	updateStateFileName    = "update-state.json"
	wtxUpdateCommandFormat = "wtx %s -> %s available. Run: wtx update"
	releaseArchiveFormat   = "wtx_%s_%s.tar.gz"
	releaseDownloadFormat  = "https://github.com/%s/releases/download/%s/%s"
)

var releaseVersionPattern = regexp.MustCompile(`^v(\d+)\.(\d+)\.(\d+)$`)
var resolveLatestVersionFn = resolveLatestVersion

type parsedVersion struct {
	Major int
	Minor int
	Patch int
}

type updateState struct {
	LastCheckedUnix int64  `json:"last_checked_unix"`
	LastSeenVersion string `json:"last_seen_version,omitempty"`
}

type updateCheckResult struct {
	CurrentVersion  string
	LatestVersion   string
	UpdateAvailable bool
	ResolveError    string
}

func runUpdateCommand(checkOnly bool, quiet bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), resolveUpdateTimeout)
	defer cancel()

	latest, err := resolveLatestVersionFn(ctx)
	if err != nil {
		return err
	}
	cur := currentVersion()

	result := updateCheckResult{
		CurrentVersion:  cur,
		LatestVersion:   latest,
		UpdateAvailable: isUpdateAvailableForInstall(cur, latest),
	}

	if checkOnly {
		printUpdateCheckResult(result, quiet)
		return nil
	}

	if !result.UpdateAvailable {
		if quiet {
			fmt.Println("up_to_date")
			return nil
		}
		fmt.Printf("wtx is up to date (%s)\n", result.CurrentVersion)
		return nil
	}

	installCtx, installCancel := context.WithTimeout(context.Background(), installUpdateTimeout)
	defer installCancel()
	stopSpinner := func() {}
	if !quiet {
		stopSpinner = startDelayedSpinner(fmt.Sprintf("Updating wtx to %s...", result.LatestVersion), 0)
	}
	defer stopSpinner()
	if err := installVersion(installCtx, result.LatestVersion); err != nil {
		return err
	}

	if quiet {
		fmt.Println(result.LatestVersion)
		return nil
	}
	fmt.Printf("Updated wtx to %s\n", result.LatestVersion)
	return nil
}

func printUpdateCheckResult(result updateCheckResult, quiet bool) {
	printUpdateCheckResultTo(os.Stdout, result, quiet)
}

func printUpdateCheckResultTo(w io.Writer, result updateCheckResult, quiet bool) {
	if quiet {
		if result.UpdateAvailable {
			fmt.Fprintln(w, result.LatestVersion)
			return
		}
		fmt.Fprintln(w, "up_to_date")
		return
	}

	if result.UpdateAvailable {
		fmt.Fprintf(w, "Update available: wtx %s -> %s\n", result.CurrentVersion, result.LatestVersion)
		return
	}
	fmt.Fprintf(w, "wtx is up to date (%s)\n", result.CurrentVersion)
}

func maybeStartInvocationUpdateCheck(args []string) {
	if !shouldRunInvocationUpdateCheck(args) {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), startupUpdateTimeout)
		defer cancel()

		result, err := checkForUpdatesWithThrottle(ctx, currentVersion(), defaultUpdateInterval)
		if err != nil || !result.UpdateAvailable {
			return
		}
		fmt.Fprintf(os.Stderr, wtxUpdateCommandFormat+"\n", result.CurrentVersion, result.LatestVersion)
	}()
}

func shouldRunInvocationUpdateCheck(args []string) bool {
	if len(args) <= 1 {
		return false
	}
	name := strings.TrimSpace(args[1])
	if name == "" {
		return true
	}
	switch name {
	case "-v", "--version", "co", "checkout", "pr", "tmux-status", "tmux-title", "tmux-agent-start", "tmux-agent-exit", "tmux-actions", "completion", "__complete", "__completeNoDesc", "update":
		return false
	default:
		return true
	}
}

func checkForUpdatesWithThrottle(ctx context.Context, currentVersion string, interval time.Duration) (updateCheckResult, error) {
	currentVersion = strings.TrimSpace(currentVersion)
	state, _ := readUpdateState()
	now := time.Now()
	cachedLatest := strings.TrimSpace(state.LastSeenVersion)
	if !shouldCheckForUpdates(state.LastCheckedUnix, now, interval) {
		return updateCheckResult{
			CurrentVersion:  currentVersion,
			LatestVersion:   cachedLatest,
			UpdateAvailable: isUpdateAvailableForInstall(currentVersion, cachedLatest),
		}, nil
	}

	latest, err := resolveLatestVersionFn(ctx)
	if err == nil {
		state.LastCheckedUnix = now.Unix()
		state.LastSeenVersion = latest
		cachedLatest = latest
		_ = writeUpdateState(state)
		return updateCheckResult{
			CurrentVersion:  currentVersion,
			LatestVersion:   latest,
			UpdateAvailable: isUpdateAvailableForInstall(currentVersion, latest),
		}, nil
	}
	if strings.TrimSpace(cachedLatest) != "" {
		state.LastCheckedUnix = now.Unix()
		_ = writeUpdateState(state)
		return updateCheckResult{
			CurrentVersion:  currentVersion,
			LatestVersion:   cachedLatest,
			UpdateAvailable: isUpdateAvailableForInstall(currentVersion, cachedLatest),
			ResolveError:    err.Error(),
		}, nil
	}
	return updateCheckResult{}, err
}

func shouldCheckForUpdates(lastCheckedUnix int64, now time.Time, interval time.Duration) bool {
	if lastCheckedUnix <= 0 {
		return true
	}
	lastChecked := time.Unix(lastCheckedUnix, 0)
	if now.Before(lastChecked) {
		return true
	}
	return now.Sub(lastChecked) >= interval
}

func resolveLatestVersion(ctx context.Context) (string, error) {
	output, err := runCommand(ctx, "git", []string{"ls-remote", "--tags", "--refs", updateRepoGitURL}, nil)
	if err != nil {
		return "", fmt.Errorf("failed to resolve latest version: %w", err)
	}
	latest, ok := latestVersionFromLSRemoteOutput(output)
	if !ok {
		return "", errors.New("failed to resolve latest version: no semver tags found")
	}
	return latest, nil
}

func installVersion(ctx context.Context, targetVersion string) error {
	targetVersion = strings.TrimSpace(targetVersion)
	if !isReleaseVersion(targetVersion) {
		return fmt.Errorf("invalid target version %q", targetVersion)
	}
	assetName, err := releaseArchiveName()
	if err != nil {
		return err
	}
	archiveURL := fmt.Sprintf(releaseDownloadFormat, updateRepoPath, targetVersion, assetName)
	checksumsURL := fmt.Sprintf(releaseDownloadFormat, updateRepoPath, targetVersion, "checksums.txt")

	tmpDir, err := os.MkdirTemp("", "wtx-update-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	archivePath := filepath.Join(tmpDir, assetName)
	checksumsPath := filepath.Join(tmpDir, "checksums.txt")
	extractedBinPath := filepath.Join(tmpDir, "wtx")

	if err := downloadFile(ctx, archiveURL, archivePath); err != nil {
		return fmt.Errorf("failed to download release archive: %w", err)
	}
	if err := downloadFile(ctx, checksumsURL, checksumsPath); err != nil {
		return fmt.Errorf("failed to download checksums: %w", err)
	}
	if err := verifyArchiveChecksum(archivePath, checksumsPath, assetName); err != nil {
		return fmt.Errorf("failed checksum verification: %w", err)
	}
	if err := extractBinaryFromTarGz(archivePath, extractedBinPath); err != nil {
		return fmt.Errorf("failed to extract archive: %w", err)
	}
	if err := replaceCurrentExecutable(extractedBinPath); err != nil {
		return fmt.Errorf("failed to install updated binary: %w", err)
	}
	return nil
}

func releaseArchiveName() (string, error) {
	goos := strings.TrimSpace(runtime.GOOS)
	switch goos {
	case "darwin", "linux":
	default:
		return "", fmt.Errorf("unsupported OS for self-update: %s", goos)
	}
	goarch := strings.TrimSpace(runtime.GOARCH)
	switch goarch {
	case "amd64", "arm64":
	default:
		return "", fmt.Errorf("unsupported architecture for self-update: %s", goarch)
	}
	return fmt.Sprintf(releaseArchiveFormat, goos, goarch), nil
}

func downloadFile(ctx context.Context, url string, targetPath string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("GET %s returned %s: %s", url, resp.Status, strings.TrimSpace(string(body)))
	}
	f, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

func verifyArchiveChecksum(archivePath string, checksumsPath string, archiveName string) error {
	checksumLine, err := checksumLineForFile(checksumsPath, archiveName)
	if err != nil {
		return err
	}
	want := strings.Fields(strings.TrimSpace(checksumLine))[0]
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	got := fmt.Sprintf("%x", h.Sum(nil))
	if !strings.EqualFold(got, want) {
		return fmt.Errorf("sha256 mismatch for %s: want %s, got %s", archiveName, want, got)
	}
	return nil
}

func checksumLineForFile(checksumsPath string, fileName string) (string, error) {
	data, err := os.ReadFile(checksumsPath)
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 2 {
			continue
		}
		candidate := filepath.Base(fields[len(fields)-1])
		if candidate == fileName {
			return line, nil
		}
	}
	return "", fmt.Errorf("checksum entry not found for %s", fileName)
}

func extractBinaryFromTarGz(archivePath string, outPath string) error {
	in, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer in.Close()
	gzReader, err := gzip.NewReader(in)
	if err != nil {
		return err
	}
	defer gzReader.Close()
	tarReader := tar.NewReader(gzReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}
		if filepath.Base(strings.TrimSpace(header.Name)) != "wtx" {
			continue
		}
		out, err := os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, tarReader); err != nil {
			out.Close()
			return err
		}
		if err := out.Close(); err != nil {
			return err
		}
		return os.Chmod(outPath, 0o755)
	}
	return errors.New("binary wtx not found in archive")
}

func replaceCurrentExecutable(newBinPath string) error {
	exePath, err := os.Executable()
	if err != nil {
		return err
	}
	if resolved, err := filepath.EvalSymlinks(exePath); err == nil && strings.TrimSpace(resolved) != "" {
		exePath = resolved
	}
	exePath = filepath.Clean(strings.TrimSpace(exePath))
	if exePath == "" {
		return errors.New("current executable path is empty")
	}
	targetDir := filepath.Dir(exePath)
	tmpPath := filepath.Join(targetDir, ".wtx-update-tmp")

	src, err := os.Open(newBinPath)
	if err != nil {
		return err
	}
	defer src.Close()
	out, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, src); err != nil {
		out.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Chmod(tmpPath, 0o755); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, exePath); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}

func shouldRetryInstallForSumDB(output string) bool {
	lower := strings.ToLower(strings.TrimSpace(output))
	if lower == "" {
		return false
	}
	if strings.Contains(lower, "sumdb") {
		return true
	}
	if strings.Contains(lower, "checksum") {
		return true
	}
	if strings.Contains(lower, "verifying") && strings.Contains(lower, "go.sum") {
		return true
	}
	return false
}

func latestVersionFromLSRemoteOutput(output string) (string, bool) {
	var bestRaw string
	var best parsedVersion
	found := false
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 2 {
			continue
		}
		ref := strings.TrimSpace(fields[1])
		if !strings.HasPrefix(ref, "refs/tags/") {
			continue
		}
		candidate := strings.TrimPrefix(ref, "refs/tags/")
		parsed, ok := parseReleaseVersion(candidate)
		if !ok {
			continue
		}
		if !found || compareReleaseVersions(parsed, best) > 0 {
			bestRaw = candidate
			best = parsed
			found = true
		}
	}
	return bestRaw, found
}

func isUpdateAvailable(currentVersion string, latestVersion string) bool {
	current, okCurrent := parseReleaseVersion(strings.TrimSpace(currentVersion))
	latest, okLatest := parseReleaseVersion(strings.TrimSpace(latestVersion))
	if !okCurrent || !okLatest {
		return false
	}
	return compareReleaseVersions(latest, current) > 0
}

func isUpdateAvailableForInstall(currentVersion string, latestVersion string) bool {
	currentVersion = strings.TrimSpace(currentVersion)
	latestVersion = strings.TrimSpace(latestVersion)
	if currentVersion == "" || latestVersion == "" {
		return false
	}
	if isUpdateAvailable(currentVersion, latestVersion) {
		return true
	}
	return !isReleaseVersion(currentVersion) && isReleaseVersion(latestVersion)
}

func isReleaseVersion(version string) bool {
	_, ok := parseReleaseVersion(version)
	return ok
}

func parseReleaseVersion(version string) (parsedVersion, bool) {
	match := releaseVersionPattern.FindStringSubmatch(strings.TrimSpace(version))
	if len(match) != 4 {
		return parsedVersion{}, false
	}
	major, err := strconv.Atoi(match[1])
	if err != nil {
		return parsedVersion{}, false
	}
	minor, err := strconv.Atoi(match[2])
	if err != nil {
		return parsedVersion{}, false
	}
	patch, err := strconv.Atoi(match[3])
	if err != nil {
		return parsedVersion{}, false
	}
	return parsedVersion{Major: major, Minor: minor, Patch: patch}, true
}

func compareReleaseVersions(a parsedVersion, b parsedVersion) int {
	if a.Major != b.Major {
		if a.Major > b.Major {
			return 1
		}
		return -1
	}
	if a.Minor != b.Minor {
		if a.Minor > b.Minor {
			return 1
		}
		return -1
	}
	if a.Patch != b.Patch {
		if a.Patch > b.Patch {
			return 1
		}
		return -1
	}
	return 0
}

func runCommand(ctx context.Context, name string, args []string, extraEnv []string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if len(extraEnv) > 0 {
		cmd.Env = append(os.Environ(), extraEnv...)
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return string(output), ctxErr
		}
		return string(output), err
	}
	return string(output), nil
}

func readUpdateState() (updateState, error) {
	path, err := updateStatePath()
	if err != nil {
		return updateState{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return updateState{}, err
	}
	var state updateState
	if err := json.Unmarshal(data, &state); err != nil {
		return updateState{}, err
	}
	return state, nil
}

func writeUpdateState(state updateState) error {
	path, err := updateStatePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	payload, err := json.Marshal(state)
	if err != nil {
		return err
	}
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, payload, 0o644); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func updateStatePath() (string, error) {
	home := strings.TrimSpace(os.Getenv("HOME"))
	if home == "" {
		return "", errors.New("HOME not set")
	}
	return filepath.Join(home, ".wtx", updateStateFileName), nil
}
