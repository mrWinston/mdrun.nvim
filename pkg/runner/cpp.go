package runner

import (

	"github.com/mrWinston/mdrun.nvim/pkg/codeblock"
	"github.com/neovim/go-client/nvim"
)

type CppRunner struct {}

func (cr *CppRunner) Run(v *nvim.Nvim, cb *codeblock.Codeblock, envVars map[string]string) ([]byte, error) {

  compiledRunner := &CompiledRunner{
  	Compiler:   "g++",
  	OutputFlag: "-o",
  	FileName:   "main.cpp",
  	Name:       "cpp",
  }
  return compiledRunner.Run(v, cb, envVars)
}


