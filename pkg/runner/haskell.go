package runner

import (
	"github.com/mrWinston/mdrun.nvim/pkg/codeblock"
	"github.com/neovim/go-client/nvim"
)

type HaskellRunner struct {}

func (cr *HaskellRunner) Run(v *nvim.Nvim, cb *codeblock.Codeblock, envVars map[string]string) ([]byte, error) {
  compiledRunner := &CompiledRunner{
  	Compiler:   "ghc",
  	OutputFlag: "-o",
  	FileName:   "main.hs",
  	Name:       "haskell",
  }
  return compiledRunner.Run(v, cb, envVars)
}

