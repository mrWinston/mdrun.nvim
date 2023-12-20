package runner

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/mrWinston/mdrun.nvim/pkg/codeblock"
	"github.com/neovim/go-client/nvim"
)

type LuaRunner struct{}

const (
	LUARUNNER_OPT_IN_NVIM = "IN_NVIM"
)

func (lu *LuaRunner) Run(v *nvim.Nvim, cb *codeblock.Codeblock, envVars map[string]string) ([]byte, error) {
	if isInNvim(cb) {
		var execResult interface{}
		err := v.ExecLua(cb.Text, execResult)
		if err != nil {
			return []byte(fmt.Sprintf("Error is: %v\n", err)), err
		}
		return []byte(fmt.Sprintf("Result is: %v\n", execResult)), err
	}
  
	var outCommand *exec.Cmd
	var err error
	tmpfile, err := os.CreateTemp(os.TempDir(), "granite.tmpfile")
	if err != nil {
		return nil, err
	}
  defer tmpfile.Close()

	_, err = tmpfile.WriteString(cb.Text)
	if err != nil {
		return nil, err
	}

  outCommand = exec.Command("lua", tmpfile.Name())
  outCommand.Env = CreateEnvArray(envVars)

  return outCommand.CombinedOutput()
}

func isInNvim(cb *codeblock.Codeblock) bool {
	return cb.Opts[LUARUNNER_OPT_IN_NVIM] == "true"
}
