package runner

import (
	"os/exec"

	"github.com/neovim/go-client/nvim"
)

type GoRunner struct{
  UseGomacro bool
}


func (gr *GoRunner) CreateCommand(v *nvim.Nvim, code string, opts map[string]string, envVars map[string]string) (*exec.Cmd, error) {
	interpreter := ""
	if gr.UseGomacro {
		interpreter = "gomacro"
	} else {
		interpreter = "go run"
	}

	cmd := &InterpretedRunner{
		Interpreter: interpreter,
		FileName:    "main.go",
		Name:        "go",
	}
	return cmd.CreateCommand(v, code, opts, envVars)
}
