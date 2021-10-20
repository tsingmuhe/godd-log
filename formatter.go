package logs

import (
	"bytes"
	"encoding/json"
	"fmt"
)

const (
	FieldKeyMsg            = "msg"
	FieldKeyLevel          = "level"
	FieldKeyTime           = "time"
	FieldKeyFunc           = "func"
	FieldKeyFile           = "file"
)

type Formatter interface {
	Format(*LogEvent) ([]byte, error)
}

type JSONFormatter struct {
}

func (f *JSONFormatter) Format(event *LogEvent) ([]byte, error) {
	data := make(Fields, len(event.Data)+5)
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

	if event.flag&(Ldate|Ltime|Lmicroseconds) != 0 {
		timestampFormat := ""
		if event.flag&Ldate != 0 {
			timestampFormat = "2006-01-02"
		}

		if event.flag&(Ltime|Lmicroseconds) != 0 {
			timestampFormat = timestampFormat + " 15:04:05"
			if event.flag&Lmicroseconds != 0 {
				timestampFormat = timestampFormat + ".000000"
			}
		}

		data[FieldKeyTime] = event.Time.Format(timestampFormat)
	}

	if event.caller != nil {
		funcVal := event.caller.Function
		fileVal := fmt.Sprintf("%s:%d", event.caller.File, event.caller.Line)

		if funcVal != "" {
			data[FieldKeyFunc] = funcVal
		}

		if fileVal != "" {
			if event.flag&Lshortfile != 0 {
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
