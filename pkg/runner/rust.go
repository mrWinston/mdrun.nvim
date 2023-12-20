package runner

import (
	"github.com/mrWinston/mdrun.nvim/pkg/codeblock"
	"github.com/neovim/go-client/nvim"
)

type RustRunner struct {}

func (cr *RustRunner) Run(v *nvim.Nvim, cb *codeblock.Codeblock, envVars map[string]string) ([]byte, error) {
  compiledRunner := &CompiledRunner{
  	Compiler:   "rustc",
  	OutputFlag: "-o",
  	FileName:   "main.rs",
  	Name:       "rust",
  }
  return compiledRunner.Run(v, cb, envVars)
}

