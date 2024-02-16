package runner

import (
	"os/exec"

	"github.com/neovim/go-client/nvim"
)

type HaskellRunner struct {}

func (cr *HaskellRunner) CreateCommand(v *nvim.Nvim, code string, opts map[string]string, envVars map[string]string) (*exec.Cmd, error) {
  compiledRunner := &CompiledRunner{
  	Compiler:   "ghc",
  	OutputFlag: "-o",
  	FileName:   "main.hs",
  	Name:       "haskell",
  }
  return compiledRunner.CreateCommand(v, code, opts, envVars)
}

