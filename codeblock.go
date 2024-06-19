package main

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/neovim/go-client/nvim"
	"github.com/samber/lo"
	"github.com/sirupsen/logrus"
	ts "github.com/smacker/go-tree-sitter"
)

type Codeblock struct {
	Language  string
	StartLine int
	EndLine   int
	StartCol  int
	EndCol    int
	Opts      map[string]string
	Text      string
	Buffer    nvim.Buffer
}

const ExtmarkNs = "codeblock_run"

const (
	CbOptWorkdir             = "CWD"
	CbOptWorkdirDockerPrefix = "docker"
	CbOptID                  = "ID"
	CbOptSource              = "SOURCE"
	CbOptLastRun             = "LAST_RUN"
)

var (
	ErrNotACodeblock = errors.New("Node is not a fenced codeblock")
	DefaultShell     = "zsh"
)

func (cb *Codeblock) GetID() string {
	if id, ok := cb.Opts[CbOptID]; ok {
		return id
	}
	return cb.Opts[CbOptSource]
}

func FindCodeblockByOpt(key string, val string, buffer nvim.Buffer) (*Codeblock, error) {
	allCodeblocks, err := GetCodeblocks(buffer)
	if err != nil {
		return nil, err
	}

	matchedBlock, _ := lo.Find(allCodeblocks, func(item *Codeblock) bool {
		if item.Opts[key] == val {
			return true
		}
		return false
	})

	return matchedBlock, nil
}


func (cb *Codeblock) Write(v *nvim.Nvim) error {

	codeLines, ok := GetBufferLines(cb.Buffer)
	if !ok {
		return fmt.Errorf("No Buffer lines for buffer %d", cb.Buffer)
	}

	findString := ""
	if cb.Opts[CbOptID] != "" {
		findString = fmt.Sprintf("%s=%s", CbOptID, cb.Opts[CbOptID])
	} else {
		findString = fmt.Sprintf("%s=%s", CbOptSource, cb.Opts[CbOptSource])
	}

	_, idx, found := lo.FindIndexOf(codeLines, func(elem string) bool {
		return strings.Contains(elem, findString)
	})
	if !found {
		return fmt.Errorf("Couldn't find codeblock in buffer lines")
	}

	cb.StartLine = idx
	cb.EndLine = -1
	for i := idx + 1; i < len(codeLines); i++ {
		if codeLines[i] == "```" {
			cb.EndLine = i
			break
		}
	}

	if cb.EndLine == -1 {
		return fmt.Errorf("Cound't find endline for codeblock")
	}

	err := NvimSetBufferLines(
		v,
		cb.Buffer,
		cb.StartLine,
		cb.EndLine+1,
		cb.GetMarkdownLines(),
	)

	return err
}

func (cb *Codeblock) SetStatus(v *nvim.Nvim, status string, highlight string) error {
	namespaceID, err := v.CreateNamespace(ExtmarkNs)
	if err != nil {
		return err
	}
	var extmarkID int
	if cb.Opts[CbOptID] != "" {
		extmarkID, err = strconv.Atoi(cb.Opts[CbOptID])
	} else {
		extmarkID, err = strconv.Atoi(cb.Opts[CbOptSource])
		extmarkID++
	}

	if err != nil {
		return err
	}

	_, err = v.SetBufferExtmark(cb.Buffer, namespaceID, cb.StartLine, 0, map[string]any{
		"id":        extmarkID,
		"virt_text": [][]any{{status, highlight}},
	})

	return err
}

func NewCodeblockFromNode(node *ts.Node, buf nvim.Buffer, sourceLines []string) (*Codeblock, error) {

	sourceCode := strings.Join(sourceLines, "\n")

	if node.Type() != "fenced_code_block" {
		return nil, ErrNotACodeblock
	}

	cb := &Codeblock{
		StartLine: int(node.StartPoint().Row),
		EndLine:   int(node.EndPoint().Row),
		StartCol:  int(node.StartPoint().Column),
		EndCol:    int(node.EndPoint().Column),
		Text:      node.Content([]byte(sourceCode)),
		Opts:      make(map[string]string),
		Buffer:    buf,
	}

	if cb.EndCol == 0 {
		// this means, we're not on the last line and the endline is one line too low
		cb.EndLine = cb.EndLine - 1
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		currentChild := node.Child(i)
		if currentChild == nil {
			continue
		}

		if currentChild.Type() == "info_string" {
			infoString := currentChild.Content([]byte(sourceCode))
			infoSplit := strings.Split(infoString, " ")
			if len(infoSplit) == 0 {
				continue
			}
			cb.Language = infoSplit[0]

			if len(infoSplit) == 1 {
				continue
			}

			for _, v := range infoSplit[1:] {
				keyValSplit := strings.Split(v, "=")
				if len(keyValSplit) != 2 {
					continue
				}
				cb.Opts[keyValSplit[0]] = keyValSplit[1]
			}

		} else if currentChild.Type() == "code_fence_content" {
			cb.Text = currentChild.Content([]byte(sourceCode))
			cb.Text = strings.TrimRight(cb.Text, "`")
		}
	}

	lines := sourceLines[cb.StartLine+1 : cb.EndLine]
	cb.Text = strings.Join(lines, "\n")
	if len(lines) != 0 {
		cb.Text = cb.Text + "\n"
	}

	return cb, nil
}

func (cb *Codeblock) GetEnvVars() map[string]string {
	sourceLines, ok := GetBufferLines(cb.Buffer)
	if !ok {
		logrus.Errorf("Couldnnt find text for buffer %v", cb.Buffer)
		return nil
	}
	return GetEnvVarsForCB(cb, sourceLines)
}

func (cb *Codeblock) GetMarkdownLines() [][]byte {
	var sb strings.Builder

	newLines := [][]byte{}

	sb.WriteString("```")
	sb.WriteString(cb.Language)

	optionKeys := []string{}
	for k := range cb.Opts {
		optionKeys = append(optionKeys, k)
	}
	slices.SortFunc(optionKeys, func(a string, b string) int {
		if a == CbOptID || a == CbOptSource {
			return -1
		}
		if b == CbOptID || b == CbOptSource {
			return 1
		}
		return strings.Compare(a, b)
	})

	for _, key := range optionKeys {
		sb.WriteString(" ")
		sb.WriteString(key)
		sb.WriteString("=")
		sb.WriteString(cb.Opts[key])

	}
	newLines = append(newLines, []byte(sb.String()))
	sb.Reset()
	for _, line := range strings.Split(cb.Text, "\n") {
		newLines = append(newLines, []byte(line))
	}
	if len(newLines[len(newLines)-1]) == 0 {
		newLines = newLines[:len(newLines)-1]
	}

	newLines = append(newLines, []byte("```"))

	return newLines
}

func getChildNodesWithType(node *ts.Node, nodeType string) []*ts.Node {
	children := []*ts.Node{}
	for i := 0; i < int(node.ChildCount()); i++ {
		currentChild := node.Child(i)
		if currentChild == nil || currentChild.Type() != nodeType {
			continue
		}
		children = append(children, currentChild)
	}

	return children
}

func GetCodeblocks(curBuf nvim.Buffer) ([]*Codeblock, error) {
	sourceLines, _ := GetBufferLines(curBuf)
	return CodeBlocksFromLines(curBuf, sourceLines)
}

func FindCodeblockUnderCursor(v *nvim.Nvim) (codeblockUnderCursor *Codeblock, err error) {
	currentWindow, err := v.CurrentWindow()
	if err != nil {
		return nil, err
	}
	currentBuffer, err := v.CurrentBuffer()
	if err != nil {
		return nil, err
	}
	codeblocks, err := GetCodeblocks(currentBuffer)
	if err != nil {
		return nil, err
	}

	cursorPosition, err := v.WindowCursor(currentWindow)

	for _, cb := range codeblocks {
		if cb.StartLine < cursorPosition[0] && cb.EndLine >= cursorPosition[0] {
			codeblockUnderCursor = cb
			break
		}
	}
	if codeblockUnderCursor == nil {
		return codeblockUnderCursor, fmt.Errorf("No codeblock found under cursor")
	}

	return codeblockUnderCursor, err
}
