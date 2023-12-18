package runner

import (
	"os/exec"

	"github.com/mrWinston/mdrun.nvim/pkg/codeblock"
)

type CodeblockRunner interface {
  GetCommand(cb *codeblock.Codeblock, envVars map[string]string) (*exec.Cmd, error)
}


