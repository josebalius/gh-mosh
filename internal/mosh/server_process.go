package mosh

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/Masterminds/semver"
)

const mostServerBinary = "mosh-server"

type serverProcess struct {
	output []byte
	cmd    *exec.Cmd
}

func newServerProcess() *serverProcess {
	return &serverProcess{}
}

func (s *serverProcess) run(ctx context.Context) error {
	s.cmd = s.processCmd(ctx)
	s.cmd.Env = os.Environ()
	output, err := s.cmd.Output()
	if err != nil {
		return err
	}
	s.output = output
	return nil
}

func (s *serverProcess) processCmd(ctx context.Context) *exec.Cmd {
	return exec.CommandContext(ctx, mostServerBinary)
}

func (s *serverProcess) connDetails() (port int64, moshKey string, err error) {
	lines := strings.Split(string(s.output), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "MOSH CONNECT") {
			parts := strings.Split(line, " ")
			if len(parts) < 4 {
				return 0, "", errors.New("invalid mosh key")
			}
			p, key := parts[2], parts[3]
			serverPort, err := strconv.ParseInt(p, 10, 64)
			if err != nil {
				return 0, "", fmt.Errorf("invalid port: %w", err)
			}
			return serverPort, key, nil
		}
	}
	return 0, "", errors.New("no mosh key found")
}

func (s *serverProcess) installed() bool {
	_, err := exec.LookPath(mostServerBinary)
	return err == nil
}

func (s *serverProcess) version(ctx context.Context) (*semver.Version, error) {
	cmd := s.processCmd(ctx)
	cmd.Args = append(cmd.Args, "--version")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(out), "\n")
	if len(lines) == 0 {
		return nil, errors.New("no output from mosh-server --version")
	}
	parts := strings.Split(lines[0], " ")
	if len(parts) < 3 {
		return nil, errors.New("unexpected output from mosh-server --version")
	}
	version := strings.TrimSuffix(parts[2], ")")
	v, err := semver.NewVersion(version)
	if err != nil {
		return nil, fmt.Errorf("invalid version from mosh-server --version: %w", err)
	}
	return v, nil
}

func (s *serverProcess) stop() error {
	if s.cmd == nil {
		return nil
	}
	if err := s.cmd.Process.Signal(os.Kill); err != nil && err != os.ErrProcessDone {
		return err
	}
	// TODO(josebalius): does this kill a detached process?
	return s.cmd.Process.Signal(os.Kill)
}
