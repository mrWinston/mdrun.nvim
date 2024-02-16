package runner

import (
	"os/exec"
	"strings"

	"github.com/neovim/go-client/nvim"
)

type JavaRunner struct{
  UseJshell bool
}

func (jr *JavaRunner) CreateCommand(v *nvim.Nvim, code string, opts map[string]string, envVars map[string]string) (*exec.Cmd, error) {
	interpreter := ""
	if jr.UseJshell {
		interpreter = "jshell"
    
    if !strings.Contains(code, "/exit") {
      code = code + "\n/exit\n"
    }
	} else {
		interpreter = "java"
	}
  selectedRunner := &InterpretedRunner{
    Interpreter: interpreter,
    FileName:    "main.java",
    Name:        "java",
  }

	return selectedRunner.CreateCommand(v, code, opts, envVars)
}
