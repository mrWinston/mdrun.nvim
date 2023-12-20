package runner

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"

	"github.com/mrWinston/mdrun.nvim/pkg/codeblock"
	"github.com/neovim/go-client/nvim"
)

type CompiledRunner struct {
  Compiler string
  OutputFlag string
  FileName string
  Name string
}

func (cr *CompiledRunner) Run(v *nvim.Nvim, cb *codeblock.Codeblock, envVars map[string]string) ([]byte, error) {
	tmpDirPath, err := os.MkdirTemp(os.TempDir(), fmt.Sprintf("mdrun_%s", cr.Name))
	if err != nil {
		return nil, err
	}
	sourcePath := path.Join(tmpDirPath, cr.FileName)
	compiledPath := path.Join(tmpDirPath, "main")

	sourceFile, err := os.Create(sourcePath)
	if err != nil {
		return nil, err
	}

  _, err = io.WriteString(sourceFile, cb.Text)

  compileOut, err := exec.Command(cr.Compiler, cr.OutputFlag, compiledPath, sourcePath).CombinedOutput()
  if err != nil {
    return compileOut, err
  }
  runCommand := exec.Command(compiledPath)  
  runCommand.Env = CreateEnvArray(envVars)

  return runCommand.CombinedOutput()
}
