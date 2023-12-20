package runner

import (
	"fmt"
	"os"

	"github.com/mrWinston/mdrun.nvim/pkg/codeblock"
	"github.com/neovim/go-client/nvim"
)

type CodeblockRunner interface {
	Run(v *nvim.Nvim, cb *codeblock.Codeblock, envVars map[string]string) ([]byte, error)
}

func CreateEnvArray(envVars map[string]string) []string {
  out := os.Environ()

	for k, v := range envVars {
		out = append(out, fmt.Sprintf("%s=%s", k, v))
	}

  return out
}
