package logs

import (
	"bytes"
	"encoding/json"
	"fmt"
)

const (
	FieldKeyMsg   = "msg"
	FieldKeyLevel = "level"
	FieldKeyTime  = "time"
	FieldKeyFunc  = "func"
	FieldKeyFile  = "file"
	FieldKeyErr   = "error"
)

type Formatter interface {
	Format(*LogEvent) ([]byte, error)
}

type JSONFormatter struct {
}

func (f *JSONFormatter) Format(event *LogEvent) ([]byte, error) {
	data := make(Fields, len(event.Data)+6)
	for k, v := range event.Data {
		switch v := v.(type) {
		case error:
			data[k] = v.Error()
		default:
			data[k] = v
		}
	}

	data[FieldKeyLevel] = event.Level.String()
	data[FieldKeyMsg] = event.Message
	data[FieldKeyTime] = event.Time.Format("2006-01-02T15:04:05.000000Z07:00")
	if event.Err != nil {
		data[FieldKeyErr] = event.Err.Error()
	}

	if event.Caller != nil {
		funcVal := event.Caller.Function
		fileVal := fmt.Sprintf("%s:%d", event.Caller.File, event.Caller.Line)

		if funcVal != "" {
			data[FieldKeyFunc] = funcVal
		}

		if fileVal != "" {
			if event.FileFlag&Shortfile != 0 {
				short := fileVal
				for i := len(fileVal) - 1; i > 0; i-- {
					if fileVal[i] == '/' {
						short = fileVal[i+1:]
						break
					}
				}
				fileVal = short
			}
			data[FieldKeyFile] = fileVal
		}
	}

	var b = &bytes.Buffer{}
	encoder := json.NewEncoder(b)
	if err := encoder.Encode(data); err != nil {
		return nil, fmt.Errorf("failed to marshal fields to JSON, %w", err)
	}
	return b.Bytes(), nil
}
