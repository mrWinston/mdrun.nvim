package main

import (
	"fmt"
	"strings"

	"github.com/neovim/go-client/nvim"
)

func CodeBlocksFromLines(buffer nvim.Buffer, lines []string) ([]*Codeblock, error) {
	inBlock := false

	startLines := []int{}
	endLines := []int{}

	for idx, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			if !inBlock {
				startLines = append(startLines, idx)
				inBlock = true
			} else {
				endLines = append(endLines, idx)
				inBlock = false
			}
		}
	}

	if len(startLines) != len(endLines) {
		return nil, fmt.Errorf("Unbalanced start and end lines for codeblocks")
	}

	cbs := []*Codeblock{}

	for i, startLineNo := range startLines {
		endLineNo := endLines[i]
		curCb := &Codeblock{
			Language:  GetLangFromStartLine(lines[startLineNo]),
			StartLine: startLineNo,
			EndLine:   endLineNo,
			StartCol:  0,
			EndCol:    0,
			Opts:      GetOptsFromStartLine(lines[startLineNo]),
			Text:      strings.Join(lines[startLineNo+1:endLineNo], "\n"),
			Buffer:    buffer,
		}
		if curCb.Text != "" {
			curCb.Text = curCb.Text + "\n"
		}
		cbs = append(cbs, curCb)
	}

	return cbs, nil
}

type section struct {
	level  int
	id     string
	start  int
	end    int
	parent *section
}

func IsLineInCb(lineNo int, codeblocks []*Codeblock) bool {
	for _, cb := range codeblocks {
		if lineNo >= cb.StartLine &&
			lineNo <= cb.EndLine {
			return true
		}
	}
	return false
}

func GetEnvVarsForCB(cb *Codeblock, lines []string) map[string]string {
	allSecs := []section{}
	allCbs, _ := CodeBlocksFromLines(cb.Buffer, lines)
	var curSec *section
	var curParent *section
	for i, line := range lines {
		if !strings.HasPrefix(line, "#") {
			continue
		}

		if IsLineInCb(i, allCbs) {
			continue
		}

		lvl := strings.Count(strings.SplitN(line, " ", 2)[0], "#")

		if curSec != nil {
			if lvl < curSec.level {
				for j := 0; j < curSec.level-lvl; j++ {
					curParent = curParent.parent
				}
			} else if lvl > curSec.level {
				curParent = curSec
			}

			curSec.end = i
			allSecs = append(allSecs, *curSec)
		}

		curSec = &section{
			level:  lvl,
			start:  i,
			parent: curParent,
		}
	}
	curSec.end = len(lines)
	allSecs = append(allSecs, *curSec)

	var cbSec *section

	for i := range allSecs {
		sec := allSecs[i]

		if cb.StartLine > sec.start && cb.EndLine < sec.end {
			cbSec = &sec
			break
		}
	}

	envMap := map[string]string{}
	for cbSec != nil {
		for _, cb := range allCbs {
			if cb.StartLine > cbSec.start &&
				cb.EndLine < cbSec.end {
				// codeblock in section
				if cb.Language == "env" {
					for _, line := range strings.Split(cb.Text, "\n") {
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
			}
		}
		cbSec = cbSec.parent
	}
	return envMap
}

func GetLangFromStartLine(line string) string {
	line = strings.TrimLeft(line, "`")

	return strings.SplitN(line, " ", 2)[0]
}

func GetOptsFromStartLine(line string) map[string]string {
	outMap := map[string]string{}

	splitted := strings.Split(line, " ")

	for _, sub := range splitted {
		keyvalsplit := strings.Split(sub, "=")
		if len(keyvalsplit) != 2 {
			continue
		}
		outMap[keyvalsplit[0]] = keyvalsplit[1]
	}

	return outMap
}
