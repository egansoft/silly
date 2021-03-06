package actions

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/egansoft/breezy/config"
	"github.com/egansoft/breezy/utils"
)

// The Action interface is a wrapper around a handler for incoming requests.
// The Handler should either not write to the writer and return an http status
// code, or write the the writer and return 200.
type Action interface {
	Handle(io.Writer, io.Reader, []string, []string) (int, error)
}

type Cmd struct {
	script string
}

type Fs struct {
	root string
}

func NewCmd(urlPath []string, line string) (Action, error) {
	varToUrlIndex := make(map[string]int)
	idx := 0
	for _, token := range urlPath {
		if _, exists := varToUrlIndex[token]; exists {
			return nil, fmt.Errorf("Duplicate var defined in %s", line)
		}
		if utils.TokenIsVar(token) {
			varToUrlIndex[token] = idx
			idx++
		}
	}

	cmdVars := utils.VarsInCmd(line)
	script := line
	for _, cmdVar := range cmdVars {
		_, exists := varToUrlIndex[cmdVar]
		if !exists {
			return nil, fmt.Errorf("Var %s used in %s but not defined in %v", cmdVar, line, urlPath)
		}

		varArg := config.ShellBind(varToUrlIndex[cmdVar])
		script = strings.Replace(script, cmdVar, varArg, -1)
	}

	cmd := &Cmd{
		script: script,
	}
	return cmd, nil
}

func (c *Cmd) Handle(w io.Writer, data io.Reader, args []string, residual []string) (int, error) {
	bashArgs := []string{config.ShellArg, config.ShellPrefix + c.script}
	allArgs := append(bashArgs, args...)

	inBuf := &bytes.Buffer{}
	outBuf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	io.Copy(inBuf, data)

	cmd := exec.Command(config.Shell, allArgs...)
	cmd.Stdin = inBuf
	cmd.Stdout = outBuf
	cmd.Stderr = errBuf
	err := cmd.Run()
	if err != nil {
		fullErr := fmt.Errorf("%v: %s", err, errBuf.String())
		return http.StatusInternalServerError, fullErr
	}

	outBuf.WriteTo(w)
	return http.StatusOK, nil
}

func NewFs(root string) (Action, error) {
	fs := &Fs{
		root: root,
	}
	return fs, nil
}

func (f *Fs) Handle(w io.Writer, data io.Reader, args []string, residual []string) (int, error) {
	pathEnd := strings.Join(residual, "/")
	path := f.root + "/" + pathEnd
	file, err := os.Open(path)
	if err != nil {
		return http.StatusNotFound, nil
	}
	defer file.Close()

	io.Copy(w, file)
	return http.StatusOK, nil
}
