package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path"
	"strconv"
	"time"

	"github.com/mrWinston/mdrun.nvim/pkg/codeblock"
	"github.com/mrWinston/mdrun.nvim/pkg/markdown"
	"github.com/mrWinston/mdrun.nvim/pkg/runner"
	"github.com/neovim/go-client/nvim"
	"github.com/neovim/go-client/nvim/plugin"
	log "github.com/sirupsen/logrus"
	ts "github.com/smacker/go-tree-sitter"
)

const EXTMARK_NS = "codeblock_run"

var GLYPHS_CLOCK_ANIMATION []string = []string{"󱑖", "󱑋", "󱑌", "󱑍", "󱑎", "󱑏", "󱑐", "󱑑", "󱑒", "󱑓", "󱑔", "󱑕"}
var GLYPH_CHECKMARK string = ""
var GLYPH_ERROR string = "󱂑"

var CodeRunners map[string]runner.CodeblockRunner = map[string]runner.CodeblockRunner{}

func errChain[T any](err error, fn func() (T, error)) (T, error) {
	var zeroValue T
	if err != nil {
		return zeroValue, err
	}
	return fn()
}

func RunCodeblock(v *nvim.Nvim, args []string) {
	log.Infof("Called RunCodeblock with args: %v", args)

	currentBuffer, err := errChain[nvim.Buffer](nil, func() (nvim.Buffer, error) { return v.CurrentBuffer() })
	currentWindow, err := errChain[nvim.Window](err, func() (nvim.Window, error) { return v.CurrentWindow() })
	lines, err := errChain[[][]byte](err, func() ([][]byte, error) { return v.BufferLines(currentBuffer, 0, -1, false) })
	cursorPosition, err := errChain[[2]int](err, func() ([2]int, error) { return v.WindowCursor(currentWindow) })

	if err != nil {
		log.Errorf("Error communicating with nvim: %v", err)
		return
	}

	sourceCode := bytes.Join(lines, []byte("\n"))
	// add a newline to the end, otherwise parsing gets weird when codeblock ends on the last line of the file
	sourceCode = append(sourceCode, []byte("\n")...)

	codeblocks, err := GetCodeblocks(sourceCode)

	if err != nil {
		log.Errorf("Error while parsing codeblocks: %v", err)
		return
	}

	var codeblockUnderCursor *codeblock.Codeblock
	for _, cb := range codeblocks {
		if cb.StartLine < cursorPosition[0] && cb.EndLine >= cursorPosition[0] {
			codeblockUnderCursor = cb
			break
		}
	}

	if codeblockUnderCursor == nil {
		return
	}

	log.Debugf("Codeblock Rows: %d, %d", codeblockUnderCursor.StartLine, codeblockUnderCursor.EndLine)

	if _, ok := codeblockUnderCursor.Opts["ID"]; !ok {
		codeblockUnderCursor.Opts["ID"] = fmt.Sprintf("%d", time.Now().UnixMilli())

		err = v.SetBufferLines(
			currentBuffer,
			codeblockUnderCursor.StartLine,
			codeblockUnderCursor.EndLine,
			false,
			codeblockUnderCursor.GetMarkdownLines(),
		)

		if err != nil {
			log.Errorf("Couldn't set nvim bufferlines: %v", err)
			return
		}
	}

	if err != nil {
		log.Errorf("unable to set extmark: %v", err)
	}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	signTickerDone := make(chan bool)
	defer func() {
		select {
		case signTickerDone <- true:
			return
		default:
			return
		}
		//v.DeleteBufferExtmark(currentBuffer, namespaceID, extmarkID)
	}()

	var targetCodeBlock *codeblock.Codeblock
	go func() {
		currentRun := 0
		for {
			select {
			case <-signTickerDone:
				return
			case <-ticker.C:
				curGlyph := GLYPHS_CLOCK_ANIMATION[currentRun%len(GLYPHS_CLOCK_ANIMATION)]
				SetExtmarkOnCodeblock(v, codeblockUnderCursor, curGlyph, "DiagnosticInfo")
				if targetCodeBlock != nil {
					SetExtmarkOnCodeblock(v, targetCodeBlock, curGlyph, "DiagnosticInfo")
				}
				currentRun++
			}
		}
	}()

	for _, cb := range codeblocks {
		id, ok := cb.Opts["SOURCE"]

		if ok && id == codeblockUnderCursor.Opts["ID"] {
			targetCodeBlock = cb
			break
		}
	}

	if targetCodeBlock == nil {
    outlanguage, ok := codeblockUnderCursor.Opts["OUT"]
    if ! ok {
      outlanguage = "out"
    }
    newCbLines := fmt.Sprintf("\n```%s SOURCE=%s\n\n```\n", outlanguage, codeblockUnderCursor.Opts["ID"])
    log.Infof("Replacing line: %d", codeblockUnderCursor.EndLine)
		v.SetBufferLines(currentBuffer, codeblockUnderCursor.EndLine, codeblockUnderCursor.EndLine, false, bytes.Split([]byte(newCbLines), []byte("\n")))

		lines, err = v.BufferLines(currentBuffer, 0, -1, false)
		if err != nil {
			log.Errorf("Unable to get buffer lines: %v", err)
			return
		}
		sourceCode = bytes.Join(lines, []byte("\n"))

		codeblocks, err = GetCodeblocks(sourceCode)
		if err != nil {
			log.Errorf("Error while parsing codeblocks: %v", err)
			return
		}

		for _, cb := range codeblocks {
			id, ok := cb.Opts["SOURCE"]

			if ok && id == codeblockUnderCursor.Opts["ID"] {
				targetCodeBlock = cb
				break
			}
		}
	}

	if targetCodeBlock == nil {
		log.Error("Output codeblock wasnt found")
		return
	}

	envVars := codeblockUnderCursor.PopulateOpts(sourceCode)
	runner, ok := CodeRunners[codeblockUnderCursor.Language]
	if !ok {
		log.Errorf("Language %s can't be executed", codeblockUnderCursor.Language)
		return
	}

	outbytes, err := runner.Run(v, codeblockUnderCursor, envVars)
  if len(outbytes) == 0 || outbytes[len(outbytes) -1 ] != 0x0a {
		outbytes = append(outbytes, 0x0a)
  }

	signTickerDone <- true
	var outGlyph string
	var outHighlight string
	if err != nil {
    log.Errorf("Go an error during Execution: %v", err)
    outbytes = append(outbytes, []byte(err.Error())...)
    outbytes = append(outbytes, 0x0a)
		outGlyph = GLYPH_ERROR
		outHighlight = "DiagnosticError"
	} else {
		outGlyph = GLYPH_CHECKMARK
		outHighlight = "DiagnosticOk"
	}

	targetCodeBlock.Text = string(outbytes)
	targetCodeBlock.Opts[codeblock.CB_OPT_LAST_RUN] = time.Now().Format(time.RFC3339)

	log.Debugf("Target Codeblock Rows: %d, %d", targetCodeBlock.StartLine, targetCodeBlock.EndLine)
	err = v.SetBufferLines(
		currentBuffer,
		targetCodeBlock.StartLine,
		targetCodeBlock.EndLine,
		false,
		targetCodeBlock.GetMarkdownLines(),
	)

	SetExtmarkOnCodeblock(v, codeblockUnderCursor, outGlyph, outHighlight)
	SetExtmarkOnCodeblock(v, targetCodeBlock, outGlyph, outHighlight)

	if err != nil {
		log.Errorf("Error writing output: %v", err)
	}
}

func SetExtmarkOnCodeblock(v *nvim.Nvim, cb *codeblock.Codeblock, text string, hlgroup string) error {
	currentBuffer, err := v.CurrentBuffer()
	if err != nil {
		return err
	}

	namespaceID, err := v.CreateNamespace(EXTMARK_NS)
	if err != nil {
		return err
	}
	var extmarkID int
	if cb.Opts[codeblock.CB_OPT_ID] != "" {
		extmarkID, err = strconv.Atoi(cb.Opts[codeblock.CB_OPT_ID])
	} else {
		extmarkID, err = strconv.Atoi(cb.Opts[codeblock.CB_OPT_SOURCE])
		extmarkID++
	}

	if err != nil {
		return err
	}

	_, err = v.SetBufferExtmark(currentBuffer, namespaceID, cb.StartLine, 0, map[string]interface{}{
		"id":        extmarkID,
		"virt_text": [][]interface{}{{text, hlgroup}},
	})

	return err
}

func GetCodeblocks(sourceCode []byte) ([]*codeblock.Codeblock, error) {
	tsparser := ts.NewParser()
	tsparser.SetLanguage(markdown.GetLanguage())
	tree, err := tsparser.ParseCtx(context.TODO(), nil, sourceCode)

	if err != nil {
		return nil, err
	}

	fencedCodeBlocksPatern := "(fenced_code_block) @cb"
	query, err := ts.NewQuery([]byte(fencedCodeBlocksPatern), markdown.GetLanguage())
	if err != nil {
		return nil, err
	}
	qc := ts.NewQueryCursor()

	qc.Exec(query, tree.RootNode())

	codeblockNodes := []*ts.Node{}

	for {
		m, ok := qc.NextMatch()
		if !ok {
			break
		}
		for _, c := range m.Captures {
			codeblockNodes = append(codeblockNodes, c.Node)
		}
	}

	codeBlocks := []*codeblock.Codeblock{}

	for _, n := range codeblockNodes {
		cb, err := codeblock.NewCodeblockFromNode(n, sourceCode)
		if err != nil {
			continue
		}
		codeBlocks = append(codeBlocks, cb)
	}
	return codeBlocks, nil
}

func main() {
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
	shRunner := &runner.ShellRunner{
		DefaultShell: "zsh",
	}
	goRunner := &runner.GoRunner{}
	luaRunner := &runner.LuaRunner{}
	cRunner := &runner.CRunner{}
  cppRunner := &runner.CppRunner{}
  rustRunner := &runner.RustRunner{}
  haskellRunner := &runner.HaskellRunner{}

	CodeRunners["sh"] = shRunner
	CodeRunners["zsh"] = shRunner
	CodeRunners["bash"] = shRunner
	CodeRunners["go"] = goRunner
	CodeRunners["lua"] = luaRunner
	CodeRunners["c"] = cRunner
	CodeRunners["c++"] = cppRunner
	CodeRunners["cpp"] = cppRunner
	CodeRunners["rust"] = rustRunner
	CodeRunners["haskell"] = haskellRunner

	plugin.Main(func(p *plugin.Plugin) error {
		p.HandleFunction(&plugin.FunctionOptions{Name: "MdrunRunCodeblock"}, RunCodeblock)
		return nil
	})
}
