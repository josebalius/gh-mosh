package mosh

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/Masterminds/semver"
)

const moshClientBinary = "mosh-client"

type clientProcess struct {
	moshKey    string
	serverAddr *net.UDPAddr

	cmd *exec.Cmd
}

func newClientProcess(moshKey string, serverAddr *net.UDPAddr) *clientProcess {
	return &clientProcess{moshKey: moshKey, serverAddr: serverAddr}
}

func (c *clientProcess) start(ctx context.Context) error {
	ipAddr, port := "127.0.0.1", strconv.Itoa(c.serverAddr.Port)

	c.cmd = c.processCmd(ctx)
	c.cmd.Args = append(c.cmd.Args, ipAddr, port)
	c.cmd.Env = os.Environ()
	c.cmd.Env = append(c.cmd.Env, "MOSH_KEY="+c.moshKey)
	c.cmd.Stdin = os.Stdin
	c.cmd.Stdout = os.Stdout
	c.cmd.Stderr = os.Stderr
	if err := c.cmd.Start(); err != nil {
		return err
	}
	return c.cmd.Wait()
}

func (c *clientProcess) processCmd(ctx context.Context) *exec.Cmd {
	return exec.CommandContext(ctx, moshClientBinary)
}

func (c *clientProcess) installed() bool {
	_, err := exec.LookPath(moshClientBinary)
	return err == nil
}

func (c *clientProcess) version(ctx context.Context) (*semver.Version, error) {
	cmd := c.processCmd(ctx)
	cmd.Args = append(cmd.Args, "--version")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(out), "\n")
	if len(lines) == 0 {
		return nil, errors.New("no output from mosh-client --version")
	}
	parts := strings.Split(lines[0], " ")
	if len(parts) < 3 {
		return nil, errors.New("unexpected output from mosh-client --version")
	}
	version := strings.TrimSuffix(parts[2], ")")
	v, err := semver.NewVersion(version)
	if err != nil {
		return nil, fmt.Errorf("invalid version from mosh-client --version: %w", err)
	}
	return v, nil
}

func (c *clientProcess) stop() error {
	if c.cmd == nil {
		return nil
	}
	if err := c.cmd.Process.Signal(os.Kill); err != nil && err != os.ErrProcessDone {
		return err
	}
	return nil
}
