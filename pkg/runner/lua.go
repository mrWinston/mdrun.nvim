package runner

import (
	"fmt"
	"os/exec"

	"github.com/neovim/go-client/nvim"
)

type LuaRunner struct{}

const (
	LUARUNNER_OPT_IN_NVIM = "IN_NVIM"
)

func (lu *LuaRunner) CreateCommand(v *nvim.Nvim, code string, opts map[string]string, envVars map[string]string) (*exec.Cmd, error) {
	if opts[LUARUNNER_OPT_IN_NVIM] == "true" {
		var execResult interface{}
		err := v.ExecLua(code, execResult)

		if err != nil {
      return exec.Command("/bin/sh", "-c", fmt.Sprintf("echo 'Got an error: %v'; exit 1",err)), nil
		}
    return exec.Command("/bin/sh", "-c", fmt.Sprintf("echo 'Result: %+v'; exit 0",execResult)), nil
	}
  
  runner := &InterpretedRunner{
  	Interpreter: "lua",
  	FileName:    "main.lua",
  	Name:        "lua",
  }

  return runner.CreateCommand(v, code, opts, envVars)
}
