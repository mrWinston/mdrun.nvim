package runner

import (
	"os/exec"

	"github.com/neovim/go-client/nvim"
)

type CppRunner struct {}

func (cr *CppRunner) CreateCommand(v *nvim.Nvim, code string, opts map[string]string, envVars map[string]string) (*exec.Cmd, error) {

  compiledRunner := &CompiledRunner{
  	Compiler:   "g++",
  	OutputFlag: "-o",
  	FileName:   "main.cpp",
  	Name:       "cpp",
  }
  return compiledRunner.CreateCommand(v, code, opts, envVars)
}


