package main

import (
	"time"

	"github.com/sirupsen/logrus"
)

type Timer struct {
  lastStop time.Time
  logger *logrus.Logger
}

func NewTimer(logger *logrus.Logger) *Timer {
  return &Timer{
    lastStop: time.Now(),
    logger: logger,
  }
}

func (t *Timer) Print(msg string) time.Duration{
  dur := time.Now().Sub(t.lastStop)
  t.logger.Infof("%s - %s", msg, dur)
  return dur
}

func (t *Timer) Restart(msg string) time.Duration{
  now := time.Now()
  dur := now.Sub(t.lastStop)
  t.logger.Infof("%s - %s", msg, dur)
  t.lastStop = now
  return dur
}
