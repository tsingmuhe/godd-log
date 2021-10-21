package logs_test

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	logs "github.com/tsingmuhe/godd-log"
)

type TestHook struct {
	Fired bool
}

func (hook *TestHook) Levels() []logs.Level {
	return []logs.Level{
		logs.TraceLevel,
		logs.DebugLevel,
		logs.InfoLevel,
		logs.WarnLevel,
		logs.ErrorLevel,
	}
}

func (hook *TestHook) Fire(event *logs.LogEvent) error {
	hook.Fired = true
	fmt.Println("Fired")
	event.WithField("logId", 1)
	return nil
}

func TestLogger_Debug(t *testing.T) {
	buf := &bytes.Buffer{}

	logger := logs.New(logs.TraceLevel, new(logs.JSONFormatter), buf)
	logger.SetFileFlag(logs.Longfile).AddHook(new(TestHook)).Start()

	logger.Info().WithField("name", "sunchp").WithFields(logs.Fields{"age": 19}).WithError(errors.New("test err")).Log("hello world")

	logger.Stop()
	fmt.Println(buf.String())
}
