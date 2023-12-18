package runner

import (
	"os"
	"os/exec"
	"path"
	"text/template"

	"github.com/mrWinston/mdrun.nvim/pkg/codeblock"
)


type GoRunner struct {}

var mainGoTpl string = `
package main

func main() {
{{ . }}
}
  `

func (gr *GoRunner) GetCommand(cb *codeblock.Codeblock, envVars map[string]string) (*exec.Cmd, error) {
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

  err = tpl.Execute(mainGo, cb.Text)
  if err != nil {
    return nil, err
  }
  err = exec.Command("goimports", "-w", mainGoPath).Run()
  if err != nil {
    return nil, err
  }
  
  outCommand = exec.Command("go", "run", mainGoPath)

  return outCommand, err
}

