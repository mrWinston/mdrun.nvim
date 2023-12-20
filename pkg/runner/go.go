package runner

import (
	"io"
	"os"
	"os/exec"
	"path"
	"text/template"

	"github.com/mrWinston/mdrun.nvim/pkg/codeblock"
	"github.com/neovim/go-client/nvim"
	log "github.com/sirupsen/logrus"
)

type GoRunner struct{}

const (
	GORUNNER_OPT_FULLFILE = "FULL_FILE"
)

var mainGoTpl string = `
package main

func main() {
{{ . }}
}
  `

func (gr *GoRunner) Run(v *nvim.Nvim, cb *codeblock.Codeblock, envVars map[string]string) ([]byte, error) {
	var outCommand *exec.Cmd
	var err error

	tmpDirPath, err := os.MkdirTemp(os.TempDir(), "mdrun_go")
	if err != nil {
		return nil, err
	}

	tpl := template.Must(template.New("mainGo").Parse(mainGoTpl))

	mainGoPath := path.Join(tmpDirPath, "main.go")

	mainGo, err := os.Create(mainGoPath)
	if err != nil {
		return nil, err
	}

	defer mainGo.Close()
	if isFullFile(cb) {
		_, err = io.WriteString(mainGo, cb.Text)
	} else {
		err = tpl.Execute(mainGo, cb.Text)
	}

	if err != nil {
		return nil, err
	}
	err = exec.Command("goimports", "-w", mainGoPath).Run()
	if err != nil {
		return nil, err
	}

	outCommand = exec.Command("go", "run", mainGoPath)
	outCommand.Env = CreateEnvArray(envVars)

	log.Debugf("Path: %s, Args: %v", outCommand.Path, outCommand.Args)

	return outCommand.CombinedOutput()
}

func isFullFile(cb *codeblock.Codeblock) bool {
	return cb.Opts[GORUNNER_OPT_FULLFILE] == "true"
}
