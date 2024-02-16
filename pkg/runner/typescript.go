package runner

import (
	"os/exec"

	"github.com/neovim/go-client/nvim"
)

type TypescriptRunner struct{}

var TYPESCRIPTRUNNER_DEFAULT_ENGINE = "deno"

func (ts *TypescriptRunner) CreateCommand(v *nvim.Nvim, code string, opts map[string]string, envVars map[string]string) (*exec.Cmd, error) {
  interpreterCommand := ""
  if TYPESCRIPTRUNNER_DEFAULT_ENGINE == "deno" {
    interpreterCommand = "deno run"
  } else {
    interpreterCommand = JSRUNNER_DEFAULT_ENGINE
  }
  run := &InterpretedRunner{
  	Interpreter: interpreterCommand,
  	FileName:    "main.ts",
  	Name:        "ts",
  }
  return run.CreateCommand(v, code, opts, envVars)
    
}
