package runner

import (
	"github.com/mrWinston/mdrun.nvim/pkg/codeblock"
	"github.com/neovim/go-client/nvim"
)

type CRunner struct {}

func (cr *CRunner) Run(v *nvim.Nvim, cb *codeblock.Codeblock, envVars map[string]string) ([]byte, error) {
  compiledRunner := &CompiledRunner{
  	Compiler:   "gcc",
  	OutputFlag: "-o",
  	FileName:   "main.c",
  	Name:       "c",
  }
  return compiledRunner.Run(v, cb, envVars)
}

