package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/neovim/go-client/nvim"
	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
)

var tickerUpdateInterval = 100 * time.Millisecond

var runningStreamers = map[string]*Streamer{}
var runningStreamersMutext = &sync.RWMutex{}

func removeStreamerWithID(id string) {
	runningStreamersMutext.Lock()
	defer runningStreamersMutext.Unlock()
	delete(runningStreamers, id)
}

// AddStreamer adds the streamer to the list of streamers, but only if it's not already running
func AddStreamer(s *Streamer) error {
	runningStreamersMutext.Lock()
	defer runningStreamersMutext.Unlock()
	_, ok := runningStreamers[s.Source.GetID()]
	if ok {
		return fmt.Errorf("codeblock with id %s already running", s.Source.Opts[CbOptID])
	}
	runningStreamers[s.Source.GetID()] = s
	return nil
}

// GetStreamerWithID returns the streamer with the given ID
func GetStreamerWithID(id string) (st *Streamer, ok bool) {
	runningStreamersMutext.RLock()
	defer runningStreamersMutext.RUnlock()
	st, ok = runningStreamers[id]
	return st, ok
}

// Kill terminates this streamer by sending SIGINT to it
func (s *Streamer) Kill() error {
	if s.Command.Process == nil {
		// command hasn't started yet
		return fmt.Errorf("Can't kill a process that has not started yet")
	}
	log.Debugf("Sending signal to kill process: %d", s.Command.Process.Pid)
	return syscall.Kill(-s.Command.Process.Pid, syscall.SIGINT)

}

// Streamer wraps the execution of an exec.Cmd and allows access to its
// stdin/out/err while it is running
type Streamer struct {
	V       *nvim.Nvim
	Source  *Codeblock
	Target  *Codeblock
	Command *exec.Cmd

	stdOutChan           chan string
	stdErrChan           chan string
	updateStatusStopChan chan int
	ticker               *time.Ticker
	writeDoneChan        chan int
	writeStopChan        chan int
	stdIn                io.Writer
}

// Send writes the given string to the stdin of the streamer
func (s *Streamer) Send(msg string) error {
  _, err := s.stdIn.Write([]byte(msg))
  return err
}

// Run starts execution of this streamer
func (s *Streamer) Run() error {
	s.stdOutChan = make(chan string)
	s.stdErrChan = make(chan string)
	s.updateStatusStopChan = make(chan int)
	s.writeDoneChan = make(chan int)
	s.writeStopChan = make(chan int)

	s.ticker = time.NewTicker(tickerUpdateInterval)
	s.Command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	stdout, err := s.Command.StdoutPipe()
	if err != nil {
		return err
	}

	stderr, err := s.Command.StderrPipe()
	if err != nil {
		return err
	}
  s.stdIn, err = s.Command.StdinPipe()
	if err != nil {
		return err
	}
	go readerToChannel(stdout, s.stdOutChan)
	go readerToChannel(stderr, s.stdErrChan)
	go s.UpdateLoop()
	go s.updateStatusLoop()

	err = s.Command.Start()
	if err != nil {
		log.Errorf("Got error starting: %v", err)
		s.updateStatusStopChan <- 0
		return err
	}
	go s.waitForCompletion()

	return nil
}

func (s *Streamer) updateStatusLoop() {
	currentRun := 0
	for {
		select {
		case <-s.updateStatusStopChan:
			return
		case <-s.ticker.C:
			curGlyph := clockAnimationGlyphs[currentRun%len(clockAnimationGlyphs)]
			err := s.Source.SetStatus(s.V, curGlyph, highlightGroupInfo)
			if err != nil {
				log.Errorf("couldn't set extmark: %v", err)
			}

			err = s.Target.SetStatus(s.V, curGlyph, highlightGroupInfo)
			if err != nil {
				log.Errorf("couldn't set extmark: %v", err)
			}

			currentRun++
		}
	}

}

func (s *Streamer) waitForCompletion() {
	log.Infof("waiting for readers to close")
	<-s.writeDoneChan
	<-s.writeDoneChan
	s.ticker.Stop()
	s.updateStatusStopChan <- 0
	s.writeStopChan <- 0

	err := s.Command.Wait()

	var outGlyph string
	var outHighlight string
	if err != nil {
		outGlyph = errorGlyph
		outHighlight = highlightGroupError
	} else {
		outGlyph = checkmarkGlyph
		outHighlight = highlightGroupOk
	}

	log.Infof("Completed wait: %s", err)


  s.Source.GetID()
  target, err := FindCodeblockByOpt(CbOptSource, s.Source.GetID(), s.Source.Buffer)
  if err != nil {
    log.Errorf("Coulnd't find target codeblock: %v", err)
    return
  }
  s.Target = target

	s.Target.Opts[CbOptLastRun] = time.Now().Format(time.RFC3339)
	s.Target.Opts["EXIT_CODE"] = fmt.Sprintf("%d", s.Command.ProcessState.ExitCode())

	err = s.Target.Write(s.V)
	if err != nil {
		log.Errorf("Error writing target: %v", err)
	}

	err = s.Target.SetStatus(s.V, outGlyph, outHighlight)
	if err != nil {
		log.Errorf("Couldn't set status on target codeblock %v", err)
	}
	err = s.Source.SetStatus(s.V, outGlyph, outHighlight)
	if err != nil {
		log.Errorf("Couldn't set status on Source codeblock %v", err)
	}
	removeStreamerWithID(s.Target.GetID())
}

func (s *Streamer) UpdateLoop() {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("Panic: %v", r)
			os.Exit(1)
		}
	}()
	for {
		select {
		case <-s.writeStopChan:
			return
		case t, ok := <-s.stdOutChan:
			if !ok {
				log.Infof("")
				s.writeDoneChan <- 0
				s.stdOutChan = nil
				break
			}
			if err := s.AddTextToTarget(t); err != nil {
				log.Errorf("Error updating text of target codeblock: %v", err)
			}

		case t, ok := <-s.stdErrChan:
			if !ok {
				s.writeDoneChan <- 0
				s.stdErrChan = nil
				break
			}
			if err := s.AddTextToTarget(t); err != nil {
				log.Errorf("Error updating text of target codeblock: %v", err)
			}
		}
	}

}

func (s *Streamer) AddTextToTarget(t string) error {
	n := time.Now()
	s.Target.Text = s.Target.Text + t
	err := s.Target.Write(s.V)
	log.Debugf("Updating text took: %s", time.Since(n).String())
	return err
}

func readerToChannel(reader io.Reader, outChannel chan<- string) {
	buf := make([]byte, 1024)
	// okay, so reading by byte is tricky, doing it linewise instead
	for {
		n, err := reader.Read(buf)
		if n == 0 {
			log.Infof("Closing Read channel due to: %v", err)
			close(outChannel)
			return
		}

		buf = lo.Replace(buf, 0xd, 0xa, -1)
		outChannel <- string(buf[0:n])
	}
}

func (s *Streamer) Finished() bool {
	return s.Command.ProcessState != nil
}

func (s *Streamer) Started() bool {
	return s.Command.Process != nil
}
