package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/mrWinston/mdrun.nvim/pkg/runner"
	"github.com/neovim/go-client/nvim"
	"github.com/neovim/go-client/nvim/plugin"
	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
)

var GLYPHS_CLOCK_ANIMATION []string = []string{"󱑖", "󱑋", "󱑌", "󱑍", "󱑎", "󱑏", "󱑐", "󱑑", "󱑒", "󱑓", "󱑔", "󱑕"}
var GLYPH_CHECKMARK string = ""
var GLYPH_ERROR string = "󱂑"
var HL_GROUP_ERROR string = "DiagnosticError"
var HL_GROUP_OK string = "DiagnosticOk"
var HL_GROUP_INFO string = "DiagnosticInfo"

var CodeRunnerConfigs *Config

const DOCKER_RUNTIME_DOCKER = "docker"
const DOCKER_RUNTIME_PODMAN = "podman"

func Configure(v *nvim.Nvim, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("Need exactly 1 argument")
	}
	config := &Config{}
	err := json.Unmarshal([]byte(args[0]), config)
	if err != nil {
		return err
	}
	CodeRunnerConfigs = config
	return nil
}

func KillCodeblock(v *nvim.Nvim, args []string) {
	cb, err := FindCodeblockUnderCursor(v)
	if err != nil {
		log.Errorf("No Codeblock under cursor found: %v", err)
	}

	id := cb.GetId()
	if id == "" {
		log.Error("Can't get cb id")
		return
	}

	streamer, ok := GetStreamerWithId(id)
	if !ok {
		log.Warnf("Didn't find a streamer associated with the codeblock")
	}

	err = streamer.Kill()
	if err != nil {
		log.Errorf("Error killing codeblock with id '%s': %v", id, err)
	}
}

func RunCodeblock(v *nvim.Nvim, args []string) {
	currentBuffer, err := v.CurrentBuffer()
	if err != nil {
		log.Errorf("Can't communicate with nvim: %v", err)
		return
	}
	codeblockUnderCursor, err := FindCodeblockUnderCursor(v)
	if err != nil {
		log.Errorf("No codeblock under cursor found: %v", err)
		return
	}

	var codeRunner runner.CodeblockRunner
	for _, rc := range CodeRunnerConfigs.RunnerConfigs {
		if lo.Contains(rc.Languages, codeblockUnderCursor.Language) {
			codeRunner = rc.Config
			break
		}
	}
	if codeRunner == nil {
		log.Errorf("Couldn't find runner for language: %s", codeblockUnderCursor.Language)
		return
	}

	if _, ok := codeblockUnderCursor.Opts["ID"]; !ok {
		codeblockUnderCursor.Opts["ID"] = fmt.Sprintf("%d", time.Now().UnixMilli())
		lines, ok := GetBufferLines(currentBuffer)
		if !ok {
			log.Errorf("Couldn't get buffer lines")
			return
		}
		err = v.SetBufferLines(currentBuffer, codeblockUnderCursor.StartLine, codeblockUnderCursor.StartLine+1, true, [][]byte{
			[]byte(fmt.Sprintf("%s %s=%s", lines[codeblockUnderCursor.StartLine], "ID", codeblockUnderCursor.Opts["ID"])),
		})
		if err != nil {
			log.Errorf("Coulnd't update source codeblock id: %v", err)
			return
		}
	}

	outlanguage, ok := codeblockUnderCursor.Opts["OUT"]
	if !ok {
		outlanguage = "out"
	}

	targetCodeBlock, err := codeblockUnderCursor.GetTargetCodeblock()

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

	targetCodeBlock.Language = outlanguage

	envVars := codeblockUnderCursor.GetEnvVars()
	if targetCodeBlock.Text != "" {
		targetCodeBlock.Text = ""
		log.Debug("Right before emptying target")
		if err := targetCodeBlock.Write(v); err != nil {
			log.Errorf("Couldn't write codeblock our: %v", err)
			return
		}

	}

	cmd, err := codeRunner.CreateCommand(v, codeblockUnderCursor.Text, codeblockUnderCursor.Opts, envVars)
	if err != nil {
		log.Errorf("Couldn't create command: %v", err)
	}

	if codeblockUnderCursor.Opts["DOCKER"] == "true" {
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

	err = AddStreamer(s)
	if err != nil {
		log.Errorf("Error adding streamer to running list: %v", err)
		return
	}

	err = s.Run()
	if err != nil {
		log.Errorf("Error starting codeblock: %v", err)
	}

}

func NewTargetCodeblock(codeblockUnderCursor *Codeblock, v *nvim.Nvim) (*Codeblock, error) {
	log.Debugf("Need to create target cb.")
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
		Text:   "",
		Buffer: codeblockUnderCursor.Buffer,
	}

	totalLines, err := v.BufferLineCount(codeblockUnderCursor.Buffer)
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

	err = v.SetBufferLines(codeblockUnderCursor.Buffer, writeLine, writeLine, false, [][]byte{[]byte(""), []byte("")})
	if err != nil {
		return nil, err
	}

	err = v.SetBufferLines(
		codeblockUnderCursor.Buffer,
		targetCodeBlock.StartLine,
		targetCodeBlock.EndLine+1,
		false,
		targetCodeBlock.GetMarkdownLines(),
	)

	if err != nil {
		return nil, err
	}

	return targetCodeBlock, nil
}

func (cb *Codeblock) GetTargetCodeblock() (*Codeblock, error) {
	if cb.Opts[CB_OPT_ID] == "" {
		return nil, fmt.Errorf("Can't get target of nodeblock without id")
	}
	codeblocks, err := GetCodeblocks(cb.Buffer)
	var target *Codeblock

	if err != nil {
		log.Errorf("Error while parsing codeblocks: %v", err)
		return nil, err
	}

	for _, currentBlock := range codeblocks {
		id, ok := currentBlock.Opts[CB_OPT_SOURCE]

		if ok && id == cb.Opts[CB_OPT_ID] {
			target = currentBlock
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
		for _, v := range CodeRunnerConfigs.RunnerConfigs {
			if lo.Contains(v.Languages, cb.Language) {
				image = v.Image
				ok = true
				break
			}
		}
		if !ok {
			return nil, fmt.Errorf("No image found for language: %s", cb.Language)
		}
	}
	log.Infof("Using docker image: %s", image)

	inDockerWorkdir := "/work"

	arguments := []string{}
	arguments = append(arguments, "run", "--rm")

	if CodeRunnerConfigs.DockerRuntime == DOCKER_RUNTIME_PODMAN {
		arguments = append(arguments, "--volume", fmt.Sprintf("%s:%s:z", originalCommand.Dir, inDockerWorkdir))
	} else {
		arguments = append(arguments, "--volume", fmt.Sprintf("%s:%s", originalCommand.Dir, inDockerWorkdir))
  }

	arguments = append(arguments, "--workdir", inDockerWorkdir)
	arguments = append(arguments, image)

	arguments = append(arguments, originalCommand.Args...)

	cmd = exec.Command(CodeRunnerConfigs.DockerRuntime, arguments...)

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

	plugin.Main(func(p *plugin.Plugin) error {

		p.HandleFunction(&plugin.FunctionOptions{Name: "MdrunRunCodeblock"}, RunCodeblock)
		p.HandleFunction(&plugin.FunctionOptions{Name: "MdrunKillCodeblock"}, KillCodeblock)
		p.HandleFunction(&plugin.FunctionOptions{Name: "MdrunConfigure"}, Configure)
		p.Handle(nvim.EventBufLines, HandleBufferLinesEvent)

		p.HandleAutocmd(&plugin.AutocmdOptions{
			Event:   "BufReadPost",
			Group:   "mdrun",
			Pattern: "*",
			Nested:  false,
		}, func() {
			curBuf, err := p.Nvim.CurrentBuffer()
			if err != nil {
				log.Errorf("Unable to get current buffer: %v", err)
				return
			}

			_, err = p.Nvim.AttachBuffer(curBuf, true, map[string]interface{}{})
			if err != nil {
				log.Errorf("Error Attching: %v", err)
				return
			}
			log.Infof("Subscribed for updates from buffer %d", curBuf)

		})

		return nil
	})
}
