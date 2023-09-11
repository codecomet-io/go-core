package exec

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"go.codecomet.dev/core/log"
	"go.codecomet.dev/core/reporter"
)

type Commander struct {
	Stdin         io.Reader
	mu            *sync.Mutex
	activeCommand *exec.Cmd
	Env           map[string]string
	bin           string
	Dir           string
	PreArgs       []string
	NoReport      bool
}

func Resolve(bin string) (string, error) {
	o, err := exec.Command("which", bin).Output()
	if err != nil {
		return "", fmt.Errorf("resolve errored with: %w", err)
	}

	out := string(o)
	out = strings.Trim(out, "\n")

	return out, nil
}

func New(defaultBin string, envBin string) *Commander {
	// This is only useful for test...
	bin := os.Getenv(envBin)
	if bin == "" {
		bin = defaultBin
	}

	execut := bin
	// XXX this is ill-designed
	if !filepath.IsAbs(bin) {
		var err error
		execut, err = os.Executable()

		if err != nil {
			reporter.CaptureException(fmt.Errorf("failed retrieving current binary information: %w", err))
			log.Fatal().Err(err).Msg("Cannot find current binary location. This is very wrong.")
		}

		execut = filepath.Join(filepath.Dir(execut), bin)

		if _, err := os.Stat(execut); err != nil {
			// Fallback to path resolution
			execut, _ = Resolve(bin)
		}
	}

	if _, err := os.Stat(execut); err != nil {
		w, _ := os.Getwd()
		reporter.CaptureException(fmt.Errorf("failed finding cli %s with pwd %s - err: %w", bin, w, err))
		log.Fatal().Str("pwd", w).Msgf("Failed finding cli %s with pwd %s - err: %s", bin, w, err)
	}

	return &Commander{
		mu:  &sync.Mutex{},
		bin: execut,
	}
}

func (com *Commander) PreExec(stdin io.Reader, args ...string) {
	args = append(com.PreArgs, args...)

	envs := []string{}
	for k, v := range com.Env {
		envs = append(envs, fmt.Sprintf("%s=%s", k, v))
	}

	log.Trace().Str("binary", com.bin).Strs("arguments", args).Strs("env", envs).Str("ctx", "exec/PreExec").Msg("Preparing Command")

	command := exec.Command(com.bin, args...) //nolint:gosec

	if com.Dir != "" {
		command.Dir = com.Dir
	}

	command.Env = append(os.Environ(), envs...)
	command.Stdin = stdin

	com.activeCommand = command
}

func (com *Commander) Attach(args ...string) error {
	var err error

	if com.Stdin != nil {
		com.PreExec(com.Stdin, args...)
	} else {
		com.PreExec(os.Stdin, args...)
	}
	_, _, err = com.ExecAndComplete() // TODO: Probably should be ExecAndWait

	if err != nil && !com.NoReport {
		reporter.CaptureException(fmt.Errorf("failed attached execution: %w", err))
		log.Error().Err(err).Msg("Attached execution failed")
	}

	return err
}

func (com *Commander) ExecAndComplete(args ...string) (bytes.Buffer, bytes.Buffer, error) {
	// prepare the command
	com.PreExec(com.Stdin, args...)

	command := com.activeCommand

	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	com.mu.Lock()
	err := command.Run()
	com.mu.Unlock()

	if err != nil {
		err = fmt.Errorf("ExecAndComplete errored: %w", err)
	}

	return stdout, stderr, err
}

func (com *Commander) ExecWithBuffer(args ...string) (io.ReadCloser, io.ReadCloser, error) {
	// prepare the command
	com.PreExec(com.Stdin, args...)

	sout, serr, err := com.ExecAndWait()

	if !com.NoReport && err != nil {
		reporter.CaptureException(fmt.Errorf("failed sub execution: %w - out: %s - err: %s", err, sout, serr))
		log.Error().Err(err).Msg("Execution failed")
	}

	return sout, serr, err
}

func (com *Commander) ExecAndWait() (io.ReadCloser, io.ReadCloser, error) {
	command := com.activeCommand

	outpipe, _ := command.StdoutPipe()
	errpipe, _ := command.StderrPipe()

	err := command.Start()
	if err != nil {
		err = fmt.Errorf("ExecAndWait errored: %w", err)
	}

	return outpipe, errpipe, err
}

func (com *Commander) Wait() error {
	command := com.activeCommand

	err := command.Wait()
	if err != nil {
		err = fmt.Errorf("Wait errored: %w", err)
	}

	return err
}
