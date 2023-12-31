package runner

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/mrWinston/mdrun.nvim/pkg/codeblock"
	"github.com/neovim/go-client/nvim"
)

const (
	SHELLRUNNER_OPT_WORKDIR               = "CWD"
	SHELLRUNNER_OPT_WORKDIR_DOCKER_PREFIX = "docker"
)

type ShellRunner struct {
	DefaultShell string
}


func (sh *ShellRunner) Run(v *nvim.Nvim,cb *codeblock.Codeblock, envVars map[string]string) ([]byte, error) {

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

	var shellCmd string
	if isRunInDocker(cb) {
		cwdSplit := strings.Split(cb.Opts[SHELLRUNNER_OPT_WORKDIR], ":")
		if len(cwdSplit) != 2 {
			return nil, fmt.Errorf("Malformed cwd entry")
		}

		var sb strings.Builder
		for k, v := range envVars {
			sb.WriteString(fmt.Sprintf(" -e '%s=%s'", k, v))
		}

		containerName := cwdSplit[1]
		if envVars[containerName] != "" {
			containerName = envVars[containerName]
		}
		shellCmd = fmt.Sprintf(
			"podman cp %[1]s %[2]s:/tmpscript && podman exec -i %[3]s %[2]s bash -c 'source /tmpscript'",
			tmpfile.Name(),
			containerName,
			sb.String(),
		)
	} else {
		shellCmd = fmt.Sprintf(
			"source %s",
			tmpfile.Name(),
		)

	}

	outCommand = exec.Command(sh.DefaultShell, "-i", "-c", shellCmd)
  outCommand.Env = CreateEnvArray(envVars)
  
	return outCommand.CombinedOutput()
}

func isRunInDocker(cb *codeblock.Codeblock) bool {
	return strings.HasPrefix(
		cb.Opts[SHELLRUNNER_OPT_WORKDIR],
		SHELLRUNNER_OPT_WORKDIR_DOCKER_PREFIX,
	)
}
