package main

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/mrWinston/mdrun.nvim/pkg/runner"
	"github.com/neovim/go-client/nvim"
	"github.com/neovim/go-client/nvim/plugin"
	log "github.com/sirupsen/logrus"
)


var GLYPHS_CLOCK_ANIMATION []string = []string{"󱑖", "󱑋", "󱑌", "󱑍", "󱑎", "󱑏", "󱑐", "󱑑", "󱑒", "󱑓", "󱑔", "󱑕"}
var GLYPH_CHECKMARK string = ""
var GLYPH_ERROR string = "󱂑"
var HL_GROUP_ERROR string = "DiagnosticError"
var HL_GROUP_OK string = "DiagnosticOk"
var HL_GROUP_INFO string = "DiagnosticInfo"

var CodeRunners map[string]runner.CodeblockRunner = map[string]runner.CodeblockRunner{}

var DEFAULT_IMAGES map[string]string = map[string]string{
	"c":          "gcc",
	"cpp":        "gcc",
	"go":         "golang",
	"haskell":    "haskell",
	"lua":        "nickblah/lua",
	"python":     "python",
	"javascript": "denoland/deno:alpine",
	"rust":       "rust",
	"typescript": "denoland/deno:alpine",
	"java":       "eclipse-temurin",
}

func RunCodeblock(v *nvim.Nvim, args []string) {
	_, err := v.AttachBuffer(0, true, map[string]interface{}{})
	if err != nil {
		log.Errorf("Error Attching: %v", err)
	}
  start := time.Now()
	codeblockUnderCursor, err := FindCodeblockUnderCursor(v)
	if err != nil {
		log.Errorf("No codeblock under cursor found: %v", err)
		return
	}
  log.Infof("Find codeblock took: %s", time.Since(start).String())

	codeRunner, ok := CodeRunners[codeblockUnderCursor.Language]
	if !ok {
		log.Errorf("Couldn't find runner for language: %s", codeblockUnderCursor.Language)
		return
	}

	if _, ok := codeblockUnderCursor.Opts["ID"]; !ok {
		codeblockUnderCursor.Opts["ID"] = fmt.Sprintf("%d", time.Now().UnixMilli())
		err = codeblockUnderCursor.Write()
		if err != nil {
			log.Errorf("Coulnd't update source codeblock id: %v", err)
			return
		}
	}

	codeBlockID := codeblockUnderCursor.Opts["ID"]

	var targetCodeBlock *Codeblock

	targetCodeBlock, err = GetTargetCodeblock(v, codeBlockID)

	if err != nil {
		log.Errorf("Error finding Target CB: %v", err)
	}

	if targetCodeBlock == nil {
		targetCodeBlock, err = NewTargetCodeblock(codeblockUnderCursor, v)
		if err != nil || targetCodeBlock == nil {
			log.Errorf("Error creating target codeblock: %v", err)
			return
		}
	}

	envVars := codeblockUnderCursor.GetEnvVars()

	targetCodeBlock.Text = ""
	if err := targetCodeBlock.Write(); err != nil {
		log.Errorf("Couldn't write codeblock our: %v", err)
		return
	}

	cmd, err := codeRunner.CreateCommand(v, codeblockUnderCursor.Text, codeblockUnderCursor.Opts, envVars)
	if err != nil {
		log.Errorf("Couldn't create command: %v", err)
	}

	if codeblockUnderCursor.Opts["DOCKER"] != "" {
		cmd, err = WrapInContainer(cmd, codeblockUnderCursor)
		if err != nil {
			log.Errorf("Error wrapping in docker : %v", err)
			return
		}
	}

	log.Infof("Running Command: %s", strings.Join(cmd.Args, " "))

	s := &Streamer{
		V:       v,
		Source:  codeblockUnderCursor,
		Target:  targetCodeBlock,
		Command: cmd,
	}

	err = s.Run()

	if err != nil {
		log.Errorf("Error starting codeblock: %v", err)
	}
}

func NewTargetCodeblock(codeblockUnderCursor *Codeblock, v *nvim.Nvim) (*Codeblock, error) {
	log.Debugf("Need to create target cb.")
	currentBuffer := nvim.Buffer(0)
	codeBlockID := codeblockUnderCursor.Opts["ID"]
	outlanguage, ok := codeblockUnderCursor.Opts["OUT"]
	if !ok {
		outlanguage = "out"
	}

	targetCodeBlock := &Codeblock{
		Language:  outlanguage,
		StartLine: -1,
		EndLine:   -1,
		StartCol:  0,
		EndCol:    0,
		Opts: map[string]string{
			"SOURCE": codeBlockID,
		},
		Text: "",
		V:    v,
	}

	totalLines, err := v.BufferLineCount(currentBuffer)
	if err != nil {
		return nil, err
	}
	writeLine := codeblockUnderCursor.EndLine + 1

	if codeblockUnderCursor.EndLine == totalLines-1 {
		writeLine = -1
		log.Debugf("Source Block is at end")
	}

	targetCodeBlock.StartLine = writeLine
	targetCodeBlock.EndLine = writeLine - 1

	err = v.SetBufferLines(currentBuffer, writeLine, writeLine, false, [][]byte{[]byte(""), []byte("")})
	if err != nil {
		return nil, err
	}

	err = targetCodeBlock.Write()
	if err != nil {
		return nil, err
	}

	return targetCodeBlock, nil
}

func GetTargetCodeblock(v *nvim.Nvim, sourceID string) (*Codeblock, error) {
	codeblocks, err := GetCodeblocks(v)
	var target *Codeblock

	if err != nil {
		log.Errorf("Error while parsing codeblocks: %v", err)
		return nil, err
	}

	for _, cb := range codeblocks {
		id, ok := cb.Opts["SOURCE"]

		if ok && id == sourceID {
			target = cb
			break
		}
	}

	return target, nil
}

func WrapInContainer(originalCommand *exec.Cmd, cb *Codeblock) (*exec.Cmd, error) {
	var cmd *exec.Cmd
	image := cb.Opts["IMAGE"]
	if image == "" {
		var ok bool
		image, ok = DEFAULT_IMAGES[cb.Language]
		if !ok {
			return nil, fmt.Errorf("No image found for language: %s", cb.Language)
		}
	}
	log.Infof("Using docker image: %s", image)

	inDockerWorkdir := "/work"

	arguments := []string{}
	arguments = append(arguments, "run", "--rm")
	arguments = append(arguments, "--volume", fmt.Sprintf("%s:%s:z", originalCommand.Dir, inDockerWorkdir))
	arguments = append(arguments, "--workdir", inDockerWorkdir)
	arguments = append(arguments, image)

	arguments = append(arguments, originalCommand.Args...)

	cmd = exec.Command("podman", arguments...)

	return cmd, nil
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("Panic: %v", r)
			os.Exit(1)
		}
	}()

	homedir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	logFilePath := path.Join(homedir, ".mdrun.log")
	f, err := os.OpenFile(
		logFilePath,
		os.O_RDWR|os.O_CREATE|os.O_APPEND,
		0666,
	)
	if err != nil {
		log.Fatalf("Can't open log file")
	}
	log.SetOutput(f)
	log.SetLevel(log.DebugLevel)
	log.Debug("hello there!")
	log.SetReportCaller(true)

	shRunner := &runner.ShellRunner{
		DefaultShell: "zsh",
	}
	CodeRunners["sh"] = shRunner
	CodeRunners["zsh"] = shRunner
	CodeRunners["bash"] = shRunner

	goRunner := &runner.GoRunner{
		UseGomacro: true,
	}
	CodeRunners["go"] = goRunner

	luaRunner := &runner.LuaRunner{}
	CodeRunners["lua"] = luaRunner

	cRunner := &runner.CRunner{}
	CodeRunners["c"] = cRunner

	cppRunner := &runner.CppRunner{}
	CodeRunners["c++"] = cppRunner
	CodeRunners["cpp"] = cppRunner

	rustRunner := &runner.RustRunner{}
	CodeRunners["rust"] = rustRunner

	haskellRunner := &runner.HaskellRunner{}
	CodeRunners["haskell"] = haskellRunner
	jsRunner := &runner.JSRunner{}
	CodeRunners["javascript"] = jsRunner

	tsRunner := &runner.TypescriptRunner{}
	CodeRunners["typescript"] = tsRunner

	pythonRunner := &runner.PythonRunner{}
	CodeRunners["python"] = pythonRunner

  javaRunner := &runner.JavaRunner{UseJshell: false}
	CodeRunners["java"] = javaRunner

	plugin.Main(func(p *plugin.Plugin) error {

		p.HandleFunction(&plugin.FunctionOptions{Name: "MdrunRunCodeblock"}, RunCodeblock)
    p.Handle(nvim.EventBufLines, HandleBufferLinesEvent)
//	err := p.RegisterHandler(nvim.EventBufLines, HandleBufferLinesEvent)
//	if err != nil {
//		log.Errorf("Error Registering: %v", err)
//	}
		return nil
	})
}
