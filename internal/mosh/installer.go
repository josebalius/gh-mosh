package mosh

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/Masterminds/semver"
)

const moshRepository = "https://github.com/mobile-shell/mosh"

type process interface {
	installed() bool
	version(context.Context) (*semver.Version, error)
}

type installer struct {
	p process
}

func newInstaller(p process) *installer {
	return &installer{p}
}

func (ins *installer) ensureCompatible(ctx context.Context) error {
	processInstalled := ins.p.installed()
	if !processInstalled {
		return ins.install(ctx)
	}
	processVersion, err := ins.p.version(ctx)
	if err != nil {
		return fmt.Errorf("failed to get process version: %w", err)
	}
	versionReq, err := semver.NewConstraint("=" + moshVersion)
	if err != nil {
		return fmt.Errorf("invalid mosh version requirement: %w", err)
	}
	if !versionReq.Check(processVersion) {
		return ins.install(ctx)
	}
	return nil
}

func (ins *installer) install(ctx context.Context) (err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, releaseDownloadURL(), nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req) // TODO(josebalius): Configure timeout
	if err != nil {
		return fmt.Errorf("failed to download mosh: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download mosh with status: %s", resp.Status)
	}
	// TODO(josebalius): Check SHA256
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read mosh download: %w", err)
	}
	dest := filepath.Join(os.TempDir(), tarballName())
	if err := os.WriteFile(dest, b, 0777); err != nil {
		return fmt.Errorf("failed to write mosh download: %w", err)
	}
	defer func() {
		if err := os.Remove(dest); err != nil {
			fmt.Println("failed to remove mosh download:", err)
		}
	}()
	extractDir, err := extractTarball(dest)
	if err != nil {
		return fmt.Errorf("failed to extract mosh download: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(extractDir); err != nil {
			fmt.Println("failed to remove mosh download extract dir:", err)
		}
	}()
	return installMosh(ctx, extractDir)
}

func extractTarball(p string) (string, error) {
	extractDir := filepath.Dir(p)
	r, err := os.Open(p)
	if err != nil {
		return "", fmt.Errorf("failed to open tarball: %w", err)
	}
	defer r.Close() // TODO(josebalius): Handle error
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return "", fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzr.Close()
	tr := tar.NewReader(gzr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("failed to read tarball: %w", err)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(filepath.Join(extractDir, hdr.Name), 0777); err != nil {
				return "", fmt.Errorf("failed to create directory: %w", err)
			}
		case tar.TypeReg:
			f, err := os.OpenFile(filepath.Join(extractDir, hdr.Name), os.O_CREATE|os.O_RDWR, 0777)
			if err != nil {
				return "", fmt.Errorf("failed to create file: %w", err)
			}
			if _, err := io.Copy(f, tr); err != nil {
				return "", fmt.Errorf("failed to copy file: %w", err)
			}
			f.Close()
		}
	}
	return filepath.Join(extractDir, packageName()), nil
}

func installMosh(ctx context.Context, p string) error {
	steps := [][]string{
		{"./configure"}, {"make"}, {"make", "install"},
	}
	for i, step := range steps {
		if err := runCmd(ctx, p, step...); err != nil {
			return fmt.Errorf("failed to run step %d: %w", i, err)
		}
	}
	return nil
}

func runCmd(ctx context.Context, p string, c ...string) error {
	if len(c) == 0 {
		return fmt.Errorf("no command provided")
	}
	cmd := exec.CommandContext(ctx, c[0])
	if len(c) > 1 {
		cmd.Args = append(cmd.Args, c[1:]...)
	}
	cmd.Dir = p
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}
	return cmd.Wait()
}

func tarballName() string {
	return fmt.Sprintf("%s.tar.gz", packageName())
}

func packageName() string {
	return fmt.Sprintf("mosh-%s", moshVersion)
}

func releaseDownloadURL() string {
	return fmt.Sprintf("%s/releases/download/%s/%s", moshRepository, packageName(), tarballName())
}
