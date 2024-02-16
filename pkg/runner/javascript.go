package runner

import (
	"os/exec"

	"github.com/neovim/go-client/nvim"
)

type JSRunner struct{}

var JSRUNNER_DEFAULT_ENGINE = "deno"

func (js *JSRunner) CreateCommand(v *nvim.Nvim, code string, opts map[string]string, envVars map[string]string) (*exec.Cmd, error) {
  interpreterCommand := ""
  if JSRUNNER_DEFAULT_ENGINE == "deno" {
    interpreterCommand = "deno run"
  } else {
    interpreterCommand = JSRUNNER_DEFAULT_ENGINE
  }
  run := &InterpretedRunner{
  	Interpreter: interpreterCommand,
  	FileName:    "main.js",
  	Name:        "js",
  }
  return run.CreateCommand(v, code, opts, envVars)
    
}
