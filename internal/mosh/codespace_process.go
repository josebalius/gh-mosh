package mosh

import (
	"bufio"
	"context"
	"io"
	"os"
	"os/exec"
	"strings"
)

type codespaceProcess struct {
	apiKey     string
	remoteAddr string

	cmd     *exec.Cmd
	reader  io.Reader
	writer  io.Writer
	outputw io.Writer // writes to stdout and writer
}

func newCodespaceProcess(apiKey, remoteAddr string) *codespaceProcess {
	reader, writer := io.Pipe()
	outputw := io.MultiWriter(os.Stdout, writer)
	return &codespaceProcess{apiKey: apiKey, remoteAddr: remoteAddr, reader: reader, writer: writer, outputw: outputw}
}

func (r *codespaceProcess) start(ctx context.Context) error {
	r.cmd = exec.CommandContext(
		ctx, "go", "run", "gh-mosh.go",
	)
	r.cmd.Env = os.Environ()
	r.cmd.Env = append(r.cmd.Env, "API_KEY="+r.apiKey)
	r.cmd.Env = append(r.cmd.Env, "REMOTE_ADDR="+r.remoteAddr)
	r.cmd.Env = append(r.cmd.Env, "SERVER=true")
	r.cmd.Stdout = r.outputw
	r.cmd.Stderr = r.outputw
	if err := r.cmd.Start(); err != nil {
		return err
	}
	return r.cmd.Wait()
}

func (r *codespaceProcess) stop() error {
	return r.cmd.Process.Signal(os.Kill)
}

func (r *codespaceProcess) moshKey(ctx context.Context) (string, error) {
	scan := bufio.NewScanner(r.reader)
	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
			if !scan.Scan() {
				return "", scan.Err() // return error if scan failed
			}

			line := scan.Text()
			if hasKey(line) {
				return parseKey(line), nil // return key if found
			}
		}
	}
}

func hasKey(text string) bool {
	return strings.HasPrefix(text, moshKeyPrefix)
}

func parseKey(text string) string {
	return strings.TrimSpace(strings.TrimPrefix(text, moshKeyPrefix))
}
