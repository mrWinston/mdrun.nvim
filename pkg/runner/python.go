package runner

import (
	"os/exec"

	"github.com/neovim/go-client/nvim"
)

type PythonRunner struct{}


func (py *PythonRunner) CreateCommand(v *nvim.Nvim, code string, opts map[string]string, envVars map[string]string) (*exec.Cmd, error) {
  run := &InterpretedRunner{
  	Interpreter: "python",
  	FileName:    "main.py",
  	Name:        "py",
  }
  return run.CreateCommand(v, code, opts, envVars)
}
