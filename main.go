package main

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/mrWinston/mdrun.nvim/pkg/codeblock"
	"github.com/mrWinston/mdrun.nvim/pkg/markdown"
	"github.com/neovim/go-client/nvim"
	"github.com/neovim/go-client/nvim/plugin"
	log "github.com/sirupsen/logrus"
	ts "github.com/smacker/go-tree-sitter"
)


var logger *log.Logger

const EXTMARK_NS = "codeblock_run"


func RunCodeblock(v *nvim.Nvim, args []string) {
	logger.Infof("Called RunCodeblock with args: %v", args)


	currentBuffer, err := v.CurrentBuffer()
	if err != nil {
		logger.Errorf("Unable to get current buffer: %v", err)
		return
	}

	currentWindow, err := v.CurrentWindow()
	if err != nil {
		logger.Errorf("Failed nvim call %v", err)
		return
	}

	lines, err := v.BufferLines(currentBuffer, 0, -1, false)
	if err != nil {
		logger.Errorf("Unable to get buffer lines: %v", err)
		return
	}

	cursorPosition, err := v.WindowCursor(currentWindow)
	if err != nil {
		logger.Errorf("Failed nvim call %v", err)
		return
	}

	sourceCode := bytes.Join(lines, []byte("\n"))

	codeblocks, err := GetCodeblocks(sourceCode)

	if err != nil {
		logger.Errorf("Error while parsing codeblocks: %v", err)
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
			logger.Errorf("Couldn't set nvim bufferlines: %v", err)
			return
		}
	}

	clockGlyphs := []string{"󱑖", "󱑋", "󱑌", "󱑍", "󱑎", "󱑏", "󱑐", "󱑑", "󱑒", "󱑓", "󱑔", "󱑕"}
	checkmarkGlyph := ""
  
	if err != nil {
		logger.Errorf("unable to set extmark: %v", err)
	}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	signTickerDone := make(chan bool)
	defer func() {
		select{
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
				curGlyph := clockGlyphs[currentRun%len(clockGlyphs)]
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
		v.SetBufferLines(currentBuffer, codeblockUnderCursor.EndLine+1, codeblockUnderCursor.EndLine+1, false, [][]byte{
			[]byte("```out SOURCE=" + codeblockUnderCursor.Opts["ID"]),
			[]byte(""),
			[]byte("```"),
			[]byte(""),
		})

		lines, err = v.BufferLines(currentBuffer, 0, -1, false)
		if err != nil {
			logger.Errorf("Unable to get buffer lines: %v", err)
			return
		}
		sourceCode = bytes.Join(lines, []byte("\n"))

		codeblocks, err = GetCodeblocks(sourceCode)
		if err != nil {
			logger.Errorf("Error while parsing codeblocks: %v", err)
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
		logger.Error("Output codeblock wasnt found")
		return
	}

	envVars := codeblockUnderCursor.PopulateOpts(sourceCode)
	command, err := codeblock.GetCommandForCodeblock(codeblockUnderCursor, envVars)
	if err != nil {
		logger.Errorf("Error Creating command: %v", err)
		return
	}

	outbytes, err := command.CombinedOutput()
	signTickerDone <- true
  errorGlyph := "󱂑"
  var outGlyph string
  var outHighlight string
	if err != nil {
    outGlyph = errorGlyph
    outHighlight = "DiagnosticError"
	} else {
    outGlyph = checkmarkGlyph
    outHighlight = "DiagnosticOk"
	}

	targetCodeBlock.Text = string(outbytes)
	targetCodeBlock.Opts[codeblock.CB_OPT_LAST_RUN] = time.Now().Format(time.RFC3339)
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
		logger.Errorf("Error writing output: %v", err)
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
    extmarkID ++
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
  logger = log.New()
  logger.Debug("hello there!")
	plugin.Main(func(p *plugin.Plugin) error {
		p.HandleFunction(&plugin.FunctionOptions{Name: "MdrunRunCodeblock"}, RunCodeblock)
		return nil
	})
}
