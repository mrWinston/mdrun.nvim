package runner

import (
	"os/exec"

	"github.com/neovim/go-client/nvim"
)

//go:generate gomodifytags -file $GOFILE -all -add-tags "json,yaml" -transform snakecase -override -w -quiet
type GoRunner struct {
	UseGomacro bool `json:"use_gomacro" yaml:"use_gomacro"`
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
	}
	return cmd.CreateCommand(v, code, opts, envVars)
}
