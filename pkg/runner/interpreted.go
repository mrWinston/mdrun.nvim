package runner

import (
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/neovim/go-client/nvim"
)

type InterpretedRunner struct {
	Interpreter string
	FileName    string
	Name        string
}

func (ir *InterpretedRunner) CreateCommand(v *nvim.Nvim, code string, opts map[string]string, envVars map[string]string) (*exec.Cmd, error) {
	var outCommand *exec.Cmd

  tmpDirPath, err := os.MkdirTemp(os.TempDir(), "mdrun_" + ir.Name)
	if err != nil {
		return nil, err
	}

  os.WriteFile(path.Join(tmpDirPath, ir.FileName), []byte(code), os.ModePerm)

	inter := strings.Split(ir.Interpreter, " ")
	inter = append(inter, "./" + ir.FileName)

	outCommand = exec.Command(inter[0], inter[1:]...)
	outCommand.Env = CreateEnvArray(envVars)
  outCommand.Dir = tmpDirPath

	return outCommand, nil
}
