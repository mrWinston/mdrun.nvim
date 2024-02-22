package runner

import (
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/neovim/go-client/nvim"
)

//go:generate gomodifytags -file $GOFILE -all -add-tags "json,yaml" -transform snakecase -override -w -quiet
type InterpretedRunner struct {
	Interpreter string `json:"interpreter" yaml:"interpreter"`
	FileName    string `json:"file_name" yaml:"file_name"`
}

func (ir *InterpretedRunner) CreateCommand(v *nvim.Nvim, code string, opts map[string]string, envVars map[string]string) (*exec.Cmd, error) {
	var outCommand *exec.Cmd

	tmpDirPath, err := os.MkdirTemp(os.TempDir(), "mdrun")
	if err != nil {
		return nil, err
	}

	os.WriteFile(path.Join(tmpDirPath, ir.FileName), []byte(code), os.ModePerm)

	inter := strings.Split(ir.Interpreter, " ")
	inter = append(inter, "./"+ir.FileName)

	outCommand = exec.Command(inter[0], inter[1:]...)
	outCommand.Env = CreateEnvArray(envVars)
	outCommand.Dir = tmpDirPath

	return outCommand, nil
}
