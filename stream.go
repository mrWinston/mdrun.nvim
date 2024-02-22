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

var TICKER_UPDATE_INTERVAL = 100 * time.Millisecond

var RunningStreamers map[string]*Streamer = map[string]*Streamer{}
var RunningStreamersMutext *sync.RWMutex = &sync.RWMutex{}

func RemoveStreamerWithId(id string) {
  RunningStreamersMutext.Lock()
  defer RunningStreamersMutext.Unlock()
  delete(RunningStreamers, id)
}

func AddStreamer(st *Streamer) error {
  RunningStreamersMutext.Lock()
  defer RunningStreamersMutext.Unlock()
  _, ok := RunningStreamers[st.Source.GetId()]
  if ok {
    return fmt.Errorf("Codeblock with id %s already running.", st.Source.Opts[CB_OPT_ID])
  }
  RunningStreamers[st.Source.GetId()] = st
  return nil
}

func GetStreamerWithId(id string) (st *Streamer, ok bool) {
  RunningStreamersMutext.RLock()
  defer RunningStreamersMutext.RUnlock()
  st, ok = RunningStreamers[id]
  return st, ok
}

func (st *Streamer) Kill() error {
  if st.Command.Process == nil {
    // command hasn't started yet
    return fmt.Errorf("Can't kill a process that has not started yet")
  }
  log.Debugf("Sending signal to kill process: %d", st.Command.Process.Pid)
  return syscall.Kill(-st.Command.Process.Pid, syscall.SIGINT)

//  return st.Command.Process.Kill()
}


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
}

func (s *Streamer) Run() error {
	s.stdOutChan = make(chan string)
	s.stdErrChan = make(chan string)
	s.updateStatusStopChan = make(chan int)
	s.writeDoneChan = make(chan int)
	s.writeStopChan = make(chan int)

	s.ticker = time.NewTicker(TICKER_UPDATE_INTERVAL)
  s.Command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	stdout, err := s.Command.StdoutPipe()
	if err != nil {
		return err
	}

	stderr, err := s.Command.StderrPipe()
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
			curGlyph := GLYPHS_CLOCK_ANIMATION[currentRun%len(GLYPHS_CLOCK_ANIMATION)]
			err := s.Source.SetStatus(s.V, curGlyph, HL_GROUP_INFO)
			if err != nil {
				log.Errorf("couldn't set extmark: %v", err)
			}

			err = s.Target.SetStatus(s.V, curGlyph, HL_GROUP_INFO)
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
		outGlyph = GLYPH_ERROR
		outHighlight = HL_GROUP_ERROR
	} else {
		outGlyph = GLYPH_CHECKMARK
		outHighlight = HL_GROUP_OK
	}

	log.Infof("Completed wait: %s", err)


	s.Source.Read()
	err = s.Source.Write(s.V)
	if err != nil {
		log.Errorf("Error writing Source: %v", err)
	}

	s.Target.Read()

	s.Target.Opts[CB_OPT_LAST_RUN] = time.Now().Format(time.RFC3339)
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
  RemoveStreamerWithId(s.Target.GetId())
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

		// for i := n - 1; i >= 0; i-- {
		// 	if buf[i] == 0xa {
		// 		line = append(line, buf[0:i+1]...)
		// 		log.Debugf("Sending for printing: '%s'", string(line))
		// 		outChannel <- string(line)
		// 		line = make([]byte, 0, 1024)
		// 		copy(line, buf[i+1:n])
		// 		break
		// 	}
		// }
	}
}

func (s *Streamer) Finished() bool {
	return s.Command.ProcessState != nil
}

func (s *Streamer) Started() bool {
	return s.Command.Process != nil
}
