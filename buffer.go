package main

import (
	"os"
	"sync"

	"github.com/neovim/go-client/nvim"
	log "github.com/sirupsen/logrus"
)

var BufferLines map[int][]string = map[int][]string{}
var bufferLinesMutex sync.RWMutex = sync.RWMutex{}

var bufLineEventsQueue chan *nvim.BufLinesEvent = make(chan *nvim.BufLinesEvent)

func init() {
	go bufferLinesEventLoop()
}

func HandleBufferLinesEvent(nv *nvim.Nvim, buf nvim.Buffer, changedtick int64, firstline int64, lastline int64, linedata []string, more bool) {

	bufLineEventsQueue <- &nvim.BufLinesEvent{
		Buffer:      buf,
		Changetick:  changedtick,
		FirstLine:   firstline,
		LastLine:    lastline,
		LineData:    linedata,
		IsMultipart: more,
	}

}

func bufferLinesEventLoop() {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("Panic: %v", r)
			os.Exit(1)
		}
	}()
	for {
		receiveBufferLine()
	}
}

func receiveBufferLine() {
	event := <-bufLineEventsQueue
	// returns a copy of the buffer
	bufLines, ok := GetBufferLines(event.Buffer)
	if !ok {
		if event.FirstLine != 0 || event.LastLine != -1 {
			log.Warnf("Got updated for an unknown buffer: %d, but it's not the initla one. ignoring", event.Buffer)
		} else {
			SetBufferLines(event.Buffer, event.LineData)
		}
		return
	}

  newLines := []string{}
  newLines = append(newLines, bufLines[:event.FirstLine]...)
	newLines = append(newLines, event.LineData...)
	newLines = append(newLines, bufLines[event.LastLine:]...)

	SetBufferLines(event.Buffer, newLines)
}

func SetBufferLines(buf nvim.Buffer, lines []string) {
	bufferLinesMutex.Lock()
	defer bufferLinesMutex.Unlock()
	BufferLines[int(buf)] = lines
}

func GetBufferLines(buf nvim.Buffer) ([]string, bool) {
	bufferLinesMutex.RLock()
	defer bufferLinesMutex.RUnlock()
	lines, ok := BufferLines[int(buf)]
	newLines := make([]string, len(lines))
	copy(newLines, lines)
	return newLines, ok
}
