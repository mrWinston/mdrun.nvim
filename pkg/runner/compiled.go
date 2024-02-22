package runner

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"

	"github.com/neovim/go-client/nvim"
)

//go:generate gomodifytags -file $GOFILE -all -add-tags "json,yaml" -transform snakecase -override -w -quiet
type CompiledRunner struct {
	Compiler   string `json:"compiler" yaml:"compiler"`
	OutputFlag string `json:"output_flag" yaml:"output_flag"`
	FileName   string `json:"file_name" yaml:"file_name"`
}

func (cr *CompiledRunner) CreateCommand(v *nvim.Nvim, code string, opts map[string]string, envVars map[string]string) (*exec.Cmd, error) {
	tmpDirPath, err := os.MkdirTemp(os.TempDir(), "mdrun")
	if err != nil {
		return nil, err
	}
	executableName := "main"
	sourcePath := path.Join(tmpDirPath, cr.FileName)

	sourceFile, err := os.Create(sourcePath)
	if err != nil {
		return nil, err
	}

	if _, err = io.WriteString(sourceFile, code); err != nil {
		return nil, err
	}

	compileCommandString := fmt.Sprintf("%s %s ./%s ./%s", cr.Compiler, cr.OutputFlag, executableName, cr.FileName)

	runCommand := exec.Command("sh", "-c", fmt.Sprintf("%s && ./%s", compileCommandString, executableName))
	runCommand.Dir = tmpDirPath
	runCommand.Env = CreateEnvArray(envVars)

	return runCommand, nil
}
