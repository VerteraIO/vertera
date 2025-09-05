package packages

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"os/exec"
)

// PackageType represents the type of package to install
type PackageType string

const (
	PackageTypeOVS            PackageType = "ovs"
	PackageTypeCloudHypervisor PackageType = "cloud-hypervisor"
)

// PackageRequest represents a package installation request
type PackageRequest struct {
	Packages []PackageType `json:"packages"`
	Version  string        `json:"version,omitempty"`
	OSVersion string       `json:"os_version,omitempty"` // el9, el10
}

// Install executes the installation for the given package type using the provided local RPM file paths.
// It runs system package managers directly and verifies installation.
func (s *Service) Install(req InstallRequest) error {
    switch req.PackageType {
    case PackageTypeOVS:
        return s.installOVS(req.Packages)
    case PackageTypeCloudHypervisor:
        return s.installCH(req.Packages)
    default:
        return fmt.Errorf("unsupported package type: %s", req.PackageType)
    }
}

func (s *Service) installOVS(pkgs []string) error {
    // If already installed, return success
    if err := exec.Command("rpm", "-q", "openvswitch").Run(); err == nil {
        return nil
    }
    if len(pkgs) > 0 {
        // dnf install -y <rpms>
        args := append([]string{"install", "-y"}, pkgs...)
        cmd := exec.Command("dnf", args...)
        cmd.Stdout = os.Stdout
        cmd.Stderr = os.Stderr
        if err := cmd.Run(); err != nil {
            return fmt.Errorf("dnf install failed: %w", err)
        }
    }
    // Enable and start service
    if err := exec.Command("systemctl", "enable", "openvswitch").Run(); err != nil {
        return fmt.Errorf("enable openvswitch failed: %w", err)
    }
    if err := exec.Command("systemctl", "start", "openvswitch").Run(); err != nil {
        return fmt.Errorf("start openvswitch failed: %w", err)
    }
    // Verify
    if err := exec.Command("bash", "-c", "command -v ovs-vsctl").Run(); err != nil {
        return fmt.Errorf("ovs-vsctl not found after installation")
    }
    return nil
}

func (s *Service) installCH(pkgs []string) error {
    // If already installed, return success
    if err := exec.Command("rpm", "-q", "cloud-hypervisor").Run(); err == nil {
        return nil
    }
    if len(pkgs) > 0 {
        args := append([]string{"-ivh"}, pkgs...)
        cmd := exec.Command("rpm", args...)
        cmd.Stdout = os.Stdout
        cmd.Stderr = os.Stderr
        if err := cmd.Run(); err != nil {
            return fmt.Errorf("rpm install failed: %w", err)
        }
    }
    // Verify
    if err := exec.Command("bash", "-c", "command -v cloud-hypervisor").Run(); err != nil {
        return fmt.Errorf("cloud-hypervisor not found after installation")
    }
    return nil
}

// PackageInfo represents information about a package
type PackageInfo struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Size     int64  `json:"size"`
	Required bool   `json:"required"`
}

// Service handles package management operations
type Service struct {
	cacheDir   string
	httpClient *http.Client
}

// NewService creates a new package service
func NewService(cacheDir string) *Service {
	return &Service{
		cacheDir:   cacheDir,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

// GetPackageInfo returns information about available packages
func (s *Service) GetPackageInfo(pkgType PackageType, version, osVersion string) ([]PackageInfo, error) {
	switch pkgType {
	case PackageTypeOVS:
		return s.getOVSPackageInfo(version, osVersion)
	case PackageTypeCloudHypervisor:
		return s.getCHPackageInfo(version)
	default:
		return nil, fmt.Errorf("unsupported package type: %s", pkgType)
	}
}

// DownloadPackage downloads a package to the cache directory
func (s *Service) DownloadPackage(info PackageInfo) (string, error) {
	// Create cache directory if it doesn't exist
	if err := os.MkdirAll(s.cacheDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create cache directory: %w", err)
	}

	filePath := filepath.Join(s.cacheDir, info.Name)

	// Check if file already exists and has correct size
	if fi, err := os.Stat(filePath); err == nil {
		if fi.Size() == info.Size && info.Size > 0 {
			return filePath, nil // Already cached
		}
	}

	// Download the file
	req, err := http.NewRequest("GET", info.URL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "VerteraIO/1.0")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to download: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Error closing response body: %v\n", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed with status: %s", resp.Status)
	}

	// Create the file
	out, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer func() {
		if err := out.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Error closing file: %v\n", err)
		}
	}()

	// Copy the response body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return filePath, nil
}

// getOVSPackageInfo returns OVS package information
func (s *Service) getOVSPackageInfo(version, osVersion string) ([]PackageInfo, error) {
	if version == "" {
		version = "3.6.0"
	}
	if osVersion == "" {
		osVersion = "el9" // default
	}

	baseURL := fmt.Sprintf("https://github.com/VerteraIO/openvswitch-rpm-build/releases/download/v%s/", version)

	packages := []PackageInfo{
		{
			Name:     fmt.Sprintf("openvswitch-selinux-policy-%s-1.%s.noarch.rpm", version, osVersion),
			URL:      baseURL + fmt.Sprintf("openvswitch-selinux-policy-%s-1.%s.noarch.rpm", version, osVersion),
			Required: false,
		},
		{
			Name:     fmt.Sprintf("openvswitch-%s-1.%s.x86_64.rpm", version, osVersion),
			URL:      baseURL + fmt.Sprintf("openvswitch-%s-1.%s.x86_64.rpm", version, osVersion),
			Required: true,
		},
		{
			Name:     fmt.Sprintf("python3-openvswitch-%s-1.%s.noarch.rpm", version, osVersion),
			URL:      baseURL + fmt.Sprintf("python3-openvswitch-%s-1.%s.noarch.rpm", version, osVersion),
			Required: false,
		},
	}

	return packages, nil
}

// getCHPackageInfo returns Cloud Hypervisor package information
func (s *Service) getCHPackageInfo(version string) ([]PackageInfo, error) {
	if version == "" {
		version = "47.0"
	}

	baseURL := fmt.Sprintf("https://github.com/VerteraIO/cloud-hypervisor-build/releases/download/v%s/", version)
	rpmName := fmt.Sprintf("cloud-hypervisor-%s.0-1.g88ffa129.el10.x86_64.rpm", version)

	packages := []PackageInfo{
		{
			Name:     rpmName,
			URL:      baseURL + rpmName,
			Required: true,
		},
	}

	return packages, nil
}

// InstallRequest represents the full installation request with downloaded packages
type InstallRequest struct {
	PackageType PackageType `json:"package_type"`
	Packages    []string    `json:"packages"` // File paths to downloaded packages
	OSVersion   string      `json:"os_version"`
}

// GenerateInstallScript generates a shell script for package installation
func (s *Service) GenerateInstallScript(req InstallRequest) (string, error) {
	var script strings.Builder

	script.WriteString("#!/bin/bash\n")
	script.WriteString("set -euo pipefail\n\n")
	script.WriteString("# Vertera package installation script\n")
	script.WriteString(fmt.Sprintf("# Package type: %s\n", req.PackageType))
	script.WriteString(fmt.Sprintf("# OS version: %s\n\n", req.OSVersion))

	switch req.PackageType {
	case PackageTypeOVS:
		script.WriteString(s.generateOVSInstallScript(req.Packages))
	case PackageTypeCloudHypervisor:
		script.WriteString(s.generateCHInstallScript(req.Packages))
	default:
		return "", fmt.Errorf("unsupported package type: %s", req.PackageType)
	}

	return script.String(), nil
}

func (s *Service) generateOVSInstallScript(packages []string) string {
	var script strings.Builder

	script.WriteString("# Install Open vSwitch packages\n")
	script.WriteString("echo \"Installing Open vSwitch...\"\n\n")

	// Check if already installed
	script.WriteString("if rpm -q openvswitch >/dev/null 2>&1; then\n")
	script.WriteString("  echo \"Open vSwitch is already installed\"\n")
	script.WriteString("  exit 0\n")
	script.WriteString("fi\n\n")

	// Install packages
	if len(packages) > 0 {
		script.WriteString("# Install RPM packages\n")
		script.WriteString(fmt.Sprintf("dnf install -y %s\n\n", strings.Join(packages, " ")))
	}

	// Start and enable service
	script.WriteString("# Start and enable Open vSwitch service\n")
	script.WriteString("systemctl start openvswitch\n")
	script.WriteString("systemctl enable openvswitch\n\n")

	// Verify installation
	script.WriteString("# Verify installation\n")
	script.WriteString("if ! command -v ovs-vsctl >/dev/null 2>&1; then\n")
	script.WriteString("  echo \"ERROR: ovs-vsctl not found after installation\"\n")
	script.WriteString("  exit 1\n")
	script.WriteString("fi\n\n")

	script.WriteString("echo \"Open vSwitch installation completed successfully\"\n")
	script.WriteString("ovs-vsctl --version\n")

	return script.String()
}

func (s *Service) generateCHInstallScript(packages []string) string {
	var script strings.Builder

	script.WriteString("# Install Cloud Hypervisor\n")
	script.WriteString("echo \"Installing Cloud Hypervisor...\"\n\n")

	// Check if already installed
	script.WriteString("if rpm -q cloud-hypervisor >/dev/null 2>&1; then\n")
	script.WriteString("  echo \"Cloud Hypervisor is already installed\"\n")
	script.WriteString("  exit 0\n")
	script.WriteString("fi\n\n")

	// Install packages
	if len(packages) > 0 {
		script.WriteString("# Install RPM package\n")
		script.WriteString(fmt.Sprintf("rpm -ivh %s\n\n", strings.Join(packages, " ")))
	}

	// Verify installation
	script.WriteString("# Verify installation\n")
	script.WriteString("if ! command -v cloud-hypervisor >/dev/null 2>&1; then\n")
	script.WriteString("  echo \"ERROR: cloud-hypervisor not found after installation\"\n")
	script.WriteString("  exit 1\n")
	script.WriteString("fi\n\n")

	script.WriteString("echo \"Cloud Hypervisor installation completed successfully\"\n")
	script.WriteString("cloud-hypervisor --version\n")

	return script.String()
}
