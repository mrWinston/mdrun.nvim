package runner

import (
	"os/exec"

	"github.com/neovim/go-client/nvim"
)

type RustRunner struct {}

func (cr *RustRunner) CreateCommand(v *nvim.Nvim, code string, opts map[string]string, envVars map[string]string) (*exec.Cmd, error) {
  compiledRunner := &CompiledRunner{
  	Compiler:   "rustc",
  	OutputFlag: "-o",
  	FileName:   "main.rs",
  	Name:       "rust",
  }
  return compiledRunner.CreateCommand(v, code, opts, envVars)
}

