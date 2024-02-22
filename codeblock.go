package main

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mrWinston/mdrun.nvim/pkg/markdown"
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
	Node      *ts.Node
	Opts      map[string]string
	Text      string
	Buffer    nvim.Buffer
}

const EXTMARK_NS = "codeblock_run"

const (
	CB_OPT_WORKDIR               = "CWD"
	CB_OPT_WORKDIR_DOCKER_PREFIX = "docker"
	CB_OPT_ID                    = "ID"
	CB_OPT_SOURCE                = "SOURCE"
	CB_OPT_LAST_RUN              = "LAST_RUN"
)

var (
	NotACodeblockError = errors.New("Node is not a fenced codeblock")
	DefaultShell       = "zsh"
)

func (cb *Codeblock) IsTarget() bool {
  _, ok := cb.Opts[CB_OPT_SOURCE] 
  return ok
}

func (cb *Codeblock) GetId() string {
  if id, ok := cb.Opts[CB_OPT_ID]; ok {
    return id
  }
  return cb.Opts[CB_OPT_SOURCE]
}


func (cb *Codeblock) Read() error {

	allCodeblocks, err := GetCodeblocks(cb.Buffer)
	if err != nil {
		return err
	}

	matchedBlock, found := lo.Find(allCodeblocks, func(item *Codeblock) bool {
		if id, ok := cb.Opts[CB_OPT_ID]; ok {
			if item.Opts[CB_OPT_ID] == id {
				return true
			}
		}
		if sid, ok := cb.Opts[CB_OPT_SOURCE]; ok {
			if item.Opts[CB_OPT_SOURCE] == sid {
				return true
			}
		}
		return false
	})

	// if we didn't find it, look for one with the same start line
	if !found {
		matchedBlock, found = lo.Find(allCodeblocks, func(item *Codeblock) bool {
			return cb.StartLine == item.StartLine
		})
		if !found {
			return fmt.Errorf("No Matching codeblock found")
		}
	}

	cb.StartLine = matchedBlock.StartLine
	cb.EndLine = matchedBlock.EndLine
	cb.StartCol = matchedBlock.StartCol
	cb.EndCol = matchedBlock.EndCol
	cb.Text = matchedBlock.Text
	cb.Node = matchedBlock.Node
	cb.Opts = matchedBlock.Opts
	cb.Language = matchedBlock.Language
	return nil
}

func (cb *Codeblock) Write(v *nvim.Nvim) error {
	n := time.Now()
	// updatedCb, err := cb.getMatchingCodeblock()
	// if err != nil {
	//   return err
	// }

  logrus.Debug("Reading lines")
	codeLines, ok := GetBufferLines(cb.Buffer)
	if !ok {
		return fmt.Errorf("No Buffer lines for buffer %d", cb.Buffer)
	}
	findString := ""
	if cb.Opts[CB_OPT_ID] != "" {
		findString = fmt.Sprintf("%s=%s", CB_OPT_ID, cb.Opts[CB_OPT_ID])
	} else {
		findString = fmt.Sprintf("%s=%s", CB_OPT_SOURCE, cb.Opts[CB_OPT_SOURCE])
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

	logrus.Debugf("Getting matching cb took: %s", time.Since(n).String())

  logrus.Debug("Writing lines")
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
	namespaceID, err := v.CreateNamespace(EXTMARK_NS)
	if err != nil {
		return err
	}
	var extmarkID int
	if cb.Opts[CB_OPT_ID] != "" {
		extmarkID, err = strconv.Atoi(cb.Opts[CB_OPT_ID])
	} else {
		extmarkID, err = strconv.Atoi(cb.Opts[CB_OPT_SOURCE])
		extmarkID++
	}

	if err != nil {
		return err
	}

	_, err = v.SetBufferExtmark(cb.Buffer, namespaceID, cb.StartLine, 0, map[string]interface{}{
		"id":        extmarkID,
		"virt_text": [][]interface{}{{status, highlight}},
	})

	return err
}

func NewCodeblockFromNode(node *ts.Node, buf nvim.Buffer, sourceLines []string) (*Codeblock, error) {

	sourceCode := strings.Join(sourceLines, "\n")

	if node.Type() != "fenced_code_block" {
		return nil, NotACodeblockError
	}

	cb := &Codeblock{
		Node:      node,
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
	envMap := map[string]string{}
	currentNode := cb.Node.Parent()
	
	sourceLines, ok := GetBufferLines(cb.Buffer)
	if !ok {
		logrus.Errorf("Couldnnt find text for buffer %v", cb.Buffer)
		return envMap
	}

	for currentNode != nil && currentNode.Type() == "section" {
		childNodes := getChildNodesWithType(currentNode, "fenced_code_block")
		for _, childNode := range childNodes {
			childCb, err := NewCodeblockFromNode(childNode, cb.Buffer, sourceLines)
			if err != nil {
				continue
			}
			if childCb.Language != "env" {
				continue
			}

			for _, line := range strings.Split(childCb.Text, "\n") {
				kvSplit := strings.Split(line, "=")
				if len(kvSplit) != 2 {
					continue
				}

				key := strings.Trim(kvSplit[0], " ")
				val := strings.Trim(kvSplit[1], " ")

				if _, ok := envMap[key]; !ok {
					envMap[key] = val
				}
			}

		}
		currentNode = currentNode.Parent()
	}

	return envMap
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
    if a == CB_OPT_ID || a == CB_OPT_SOURCE {
      return -1
    }
    if b == CB_OPT_ID || b == CB_OPT_SOURCE {
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
	sourceCode := strings.Join(sourceLines, "\n")

	tsparser := ts.NewParser()
	tsparser.SetLanguage(markdown.GetLanguage())
	tree, err := tsparser.ParseCtx(context.TODO(), nil, []byte(sourceCode))

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

	codeBlocks := []*Codeblock{}

	for _, n := range codeblockNodes {
		cb, err := NewCodeblockFromNode(n, curBuf, sourceLines)
		if err != nil {
			continue
		}
		codeBlocks = append(codeBlocks, cb)
	}
	return codeBlocks, nil
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

	return
}
