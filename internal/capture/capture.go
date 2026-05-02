package capture

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"syscall"
	"time"

	"github.com/creack/pty"
)

type Options struct {
	Width   int
	Height  int
	Timeout time.Duration
}

type Result struct {
	Output   []byte
	ExitCode int
	TimedOut bool
}

func Run(ctx context.Context, argv []string, opts Options) (*Result, error) {
	if len(argv) == 0 {
		return nil, errors.New("capture: empty command")
	}
	if opts.Width <= 0 {
		opts.Width = 120
	}
	if opts.Height <= 0 {
		opts.Height = 40
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 30 * time.Second
	}

	runCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, argv[0], argv[1:]...)
	cmd.Env = append(cmd.Environ(),
		"TERM=xterm-256color",
		"COLORTERM=truecolor",
		"CLICOLOR=1",
		"CLICOLOR_FORCE=1",
		"FORCE_COLOR=1",
		fmt.Sprintf("COLUMNS=%d", opts.Width),
		fmt.Sprintf("LINES=%d", opts.Height),
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{
		Cols: uint16(opts.Width),
		Rows: uint16(opts.Height),
	})
	if err != nil {
		return nil, fmt.Errorf("capture: start pty: %w", err)
	}
	defer ptmx.Close()

	var buf bytes.Buffer
	_, copyErr := io.Copy(&buf, ptmx)
	waitErr := cmd.Wait()

	res := &Result{Output: buf.Bytes()}
	if cmd.ProcessState != nil {
		res.ExitCode = cmd.ProcessState.ExitCode()
	}
	if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		res.TimedOut = true
	}

	if copyErr != nil && !isExpectedReadErr(copyErr) {
		return res, fmt.Errorf("capture: read pty: %w", copyErr)
	}
	if waitErr != nil {
		if _, ok := errors.AsType[*exec.ExitError](waitErr); !ok {
			return res, fmt.Errorf("capture: wait: %w", waitErr)
		}
	}
	return res, nil
}

func isExpectedReadErr(err error) bool {
	return err == nil || errors.Is(err, io.EOF) || errors.Is(err, syscall.EIO)
}
