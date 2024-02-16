package main

import (
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/neovim/go-client/nvim"
	log "github.com/sirupsen/logrus"
)

var TICKER_UPDATE_INTERVAL = 100 * time.Millisecond

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
			err := s.Source.SetStatus(curGlyph, HL_GROUP_INFO)
			if err != nil {
				log.Errorf("couldn't set extmark: %v", err)
			}

			err = s.Target.SetStatus(curGlyph, HL_GROUP_INFO)
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
  log.Infof("first one done")
	<-s.writeDoneChan
  log.Infof("second one done.")
	s.ticker.Stop()
  log.Infof("ticket stopped")
	s.updateStatusStopChan <- 0
  log.Infof("stop update status")
	s.writeStopChan <- 0
  log.Infof("stopped write")

  log.Infof("waiting for readers to close")
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

	s.Target.Opts[CB_OPT_LAST_RUN] = time.Now().Format(time.RFC3339)
	s.Source.Opts[CB_OPT_LAST_RUN] = time.Now().Format(time.RFC3339)

	s.Source.Read()
	err = s.Source.Write()
	if err != nil {
		log.Errorf("Error writing Source: %v", err)
	}

	s.Target.Read()
	err = s.Target.Write()
	if err != nil {
		log.Errorf("Error writing target: %v", err)
	}

	err = s.Target.SetStatus(outGlyph, outHighlight)
	if err != nil {
		log.Errorf("Couldn't set status on target codeblock %v", err)
	}
	err = s.Source.SetStatus(outGlyph, outHighlight)
	if err != nil {
		log.Errorf("Couldn't set status on Source codeblock %v", err)
	}
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
	s.Target.Text = s.Target.Text + t
	err := s.Target.Write()
	return err
}

func readerToChannel(reader io.Reader, outChannel chan<- string) {
	buf := make([]byte, 1024)
	// okay, so reading by byte is tricky, doing it linewise instead
	line := make([]byte, 0, 256)
	for {
		n, err := reader.Read(buf)
		if n == 0 {
			log.Infof("Closing Read channel due to: %v", err)
			close(outChannel)
			return
		}

		for i := 0; i < n; i++ {
      if buf[i] == 0xd {
        buf[i] = 0xa
      }
			line = append(line, buf[i])
			if buf[i] == 0xa {
				outChannel <- string(line)
				line = make([]byte,0, 256)
			}
		}
	}
}

func (s *Streamer) Finished() bool {
	return s.Command.ProcessState != nil
}

func (s *Streamer) Started() bool {
	return s.Command.Process != nil
}
