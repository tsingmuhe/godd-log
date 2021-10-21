package logs

import (
	"context"
	"reflect"
	"runtime"
	"time"
)

type Fields map[string]interface{}

type LogEvent struct {
	logger *Logger

	FileFlag uint32

	Level Level

	Data Fields

	Message string

	Err error

	Time time.Time

	Caller *runtime.Frame

	Context context.Context
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

	event.Err = err
	return event
}

func (event *LogEvent) Log(msg string) {
	if event == nil {
		return
	}

	event.FileFlag = event.logger.getFileFlag()
	event.Message = msg
	event.Time = time.Now()

	if event.FileFlag&(Shortfile|Longfile) != 0 {
		event.Caller = getCaller()
	}

	event.logger.log(event)
}
