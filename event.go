package logs

import (
	"context"
	"reflect"
	"runtime"
	"sync"
	"time"
)

const (
	ErrorKey               = "error"
	maximumCallerDepth int = 25
	knownLogrusFrames  int = 3
)

var (
	logPackage         string
	minimumCallerDepth int
	callerInitOnce     sync.Once
)

type Fields map[string]interface{}

type LogEvent struct {
	logger *Logger

	flag uint32

	caller *runtime.Frame

	Level Level

	Data Fields

	Message string

	Time time.Time

	Context context.Context
}

func NewLogEvent(logger *Logger, level Level) *LogEvent {
	return &LogEvent{
		logger: logger,
		Level:  level,
		Data:   make(Fields, 6),
	}
}

func (event *LogEvent) WithField(key string, value interface{}) *LogEvent {
	if event == nil {
		return nil
	}

	return event.WithFields(Fields{key: value})
}

func (event *LogEvent) WithFields(fields Fields) *LogEvent {
	if event == nil {
		return nil
	}

	data := make(Fields, len(event.Data)+len(fields))
	for k, v := range event.Data {
		data[k] = v
	}

	for k, v := range fields {
		isErrField := false

		if t := reflect.TypeOf(v); t != nil {
			switch {
			case t.Kind() == reflect.Func, t.Kind() == reflect.Ptr && t.Elem().Kind() == reflect.Func:
				isErrField = true
			}
		}

		if !isErrField {
			data[k] = v
		}
	}

	event.Data = data
	return event
}

func (event *LogEvent) WithError(err error) *LogEvent {
	if event == nil {
		return nil
	}

	return event.WithField(ErrorKey, err)
}

func (event *LogEvent) WithContext(ctx context.Context) *LogEvent {
	if event == nil {
		return nil
	}

	event.Context = ctx
	return event
}

func (event *LogEvent) Log(msg string) {
	if event == nil {
		return
	}

	event.flag = event.logger.GetFlags()
	event.Message = msg
	event.Time = time.Now()

	if event.flag&(Lshortfile|Llongfile) != 0 {
		event.caller = getCaller()
	}

	event.logger.log(event)
}
