package main

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/neovim/go-client/nvim"
	log "github.com/sirupsen/logrus"
)

var BufferLines map[int][]string = map[int][]string{}
var bufferLinesMutex sync.RWMutex = sync.RWMutex{}

var bufLineEventsQueue chan *nvim.BufLinesEvent = make(chan *nvim.BufLinesEvent)
var bufferUnblockedChannels map[int]*atomic.Int32 = map[int]*atomic.Int32{}

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
	lines, ok := BufferLines[int(event.Buffer)]
	bufLines := make([]string, len(lines))
	copy(bufLines, lines)
	log.Debugf("Got bufferline update for buf %d", event.Buffer)

	if !ok {
		if event.FirstLine != 0 || event.LastLine != -1 {
			log.Warnf("Got updated for an unknown buffer: %d, but it's not the initla one. ignoring", event.Buffer)
		} else {
			SetBufferLines(event.Buffer, event.LineData)
		}
    blockCount := &atomic.Int32{}
		bufferUnblockedChannels[int(event.Buffer)] = blockCount
		return
	}

	newLines := []string{}
	newLines = append(newLines, bufLines[:event.FirstLine]...)
	newLines = append(newLines, event.LineData...)
	newLines = append(newLines, bufLines[event.LastLine:]...)

	SetBufferLines(event.Buffer, newLines)
	blockCount, ok := bufferUnblockedChannels[int(event.Buffer)]
	if !ok {
    blockCount = &atomic.Int32{}
		bufferUnblockedChannels[int(event.Buffer)] = blockCount
	}

  blockCount.CompareAndSwap(0, 1)
  blockCount.Add(-1)
}

func SetBufferLines(buf nvim.Buffer, lines []string) {
	bufferLinesMutex.Lock()
	defer bufferLinesMutex.Unlock()
	BufferLines[int(buf)] = lines
}

func NvimSetBufferLines(v *nvim.Nvim, buf nvim.Buffer, startLine int, endLine int, lines [][]byte) error {
	log.Debugf("Sending lines, then waiting for buf %d to receive", buf)

  blockCtr, ok := bufferUnblockedChannels[int(buf)]
  if ! ok {
    return fmt.Errorf("Writing to buffer that's not initialized yet")
  }

  blockCtr.Add(1)
	err := v.SetBufferLines(
		buf,
		startLine,
		endLine,
		false,
		lines,
	)
	log.Debug("unblocked, returning")
	return err
}

func GetBufferLines(buf nvim.Buffer) ([]string, bool) {
  log.Debugf("Getting buflines")
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("Panic: %v", r)
			os.Exit(1)
		}
	}()
  blockPtr := bufferUnblockedChannels[int(buf)]
  for blockPtr.Load() != 0 {
    log.Debug("Waiting for blockPtr to be 0")
    time.Sleep(10 * time.Millisecond)
  }
	bufferLinesMutex.RLock()
	defer bufferLinesMutex.RUnlock()
	lines, ok := BufferLines[int(buf)]
	newLines := make([]string, len(lines))
	copy(newLines, lines)
	return newLines, ok
}