package update

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

type Release struct {
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
	Body    string `json:"body"`
}

type Manager struct {
	currentVersion string
	repoOwner      string
	repoName       string
}

func NewManager() *Manager {
	return &Manager{
		currentVersion: "0.2.6",
		repoOwner:      "delta-code",
		repoName:       "cli",
	}
}

func (m *Manager) Check() (*Release, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", m.repoOwner, m.repoName)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot check for updates: %w", err)
	}
	defer resp.Body.Close()

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	return &release, nil
}

func (m *Manager) Update() error {
	fmt.Println("Checking for updates...")
	release, err := m.Check()
	if err != nil {
		return err
	}

	if release.TagName <= m.currentVersion {
		fmt.Printf("Already at latest version (%s)\n", m.currentVersion)
		return nil
	}

	fmt.Printf("New version available: %s\n", release.TagName)
	fmt.Printf("Release: %s\n", release.Name)
	fmt.Printf("Downloading update...\n")

	exe, _ := os.Executable()
	src := filepath.Dir(exe)

	// Download latest binary
	arch := runtime.GOARCH
	osName := runtime.GOOS
	url := fmt.Sprintf("https://github.com/%s/%s/releases/download/%s/delta-%s-%s",
		m.repoOwner, m.repoName, release.TagName, osName, arch)

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	tmpPath := filepath.Join(src, ".delta-update-tmp")
	f, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.ReadFrom(resp.Body)
	if err != nil {
		return err
	}
	f.Close()

	os.Chmod(tmpPath, 0755)
	if err := exec.Command("mv", tmpPath, exe).Run(); err != nil {
		os.Rename(tmpPath, exe)
	}

	fmt.Println("✓ Update complete. Restart Delta Code.")
	return nil
}
