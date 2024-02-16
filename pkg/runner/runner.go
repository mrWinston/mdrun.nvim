package runner

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/neovim/go-client/nvim"
)

type CodeblockRunner interface {
	CreateCommand(v *nvim.Nvim, code string, opts map[string]string, envVars map[string]string) (*exec.Cmd, error)
}

func CreateEnvArray(envVars map[string]string) []string {
  out := os.Environ()

	for k, v := range envVars {
		out = append(out, fmt.Sprintf("%s=%s", k, v))
	}

  return out
}

func CreateTmpFile(filename string, text string) (mainFilePath string, err error) {
  splitted := strings.Split(filename, ".")
  suffix := splitted[len(splitted) - 1]
	tmpDirPath, err := os.MkdirTemp(os.TempDir(), "mdrun_" + suffix)
	if err != nil {
		return "", err
	}

	mainFilePath = path.Join(tmpDirPath, filename)

	main, err := os.Create(mainFilePath)
	if err != nil {
		return "", err
	}
	defer main.Close()
	_, err = io.WriteString(main, text)
  
  return mainFilePath, err
}
