package runner

import (
	"os/exec"

	"github.com/neovim/go-client/nvim"
)

type CRunner struct{}

func (cr *CRunner) CreateCommand(v *nvim.Nvim, code string, opts map[string]string, envVars map[string]string) (*exec.Cmd, error) {
	compiledRunner := &CompiledRunner{
		Compiler:   "gcc",
		OutputFlag: "-o",
		FileName:   "main.c",
		Name:       "c",
	}
	return compiledRunner.CreateCommand(v, code, opts, envVars)
}
