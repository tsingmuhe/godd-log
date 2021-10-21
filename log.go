package logs

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
)

const (
	defaultAsyncMsgLen = 1e3

	Longfile uint32 = 1 << iota
	Shortfile
)

type Logger struct {
	level     Level
	eventPool sync.Pool

	fileFlag uint32

	running    uint32
	msgChanLen uint32
	msgChan    chan *LogEvent
	signalChan chan string
	wg         sync.WaitGroup

	mu        sync.RWMutex
	hooks     LevelHooks
	formatter Formatter
	out       io.Writer
}

func New(level Level, formatter Formatter, out io.Writer) *Logger {
	return &Logger{
		level:     level,
		formatter: formatter,
		out:       out,
		hooks:     make(LevelHooks),
	}
}

func (l *Logger) SetLevel(level Level) *Logger {
	atomic.StoreUint32((*uint32)(&l.level), uint32(level))
	return l
}

func (l *Logger) SetFileFlag(flag uint32) *Logger {
	atomic.StoreUint32(&l.fileFlag, flag)
	return l
}

func (l *Logger) AddHook(hook Hook) *Logger {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.hooks.Add(hook)
	return l
}

func (l *Logger) SetFormatter(formatter Formatter) *Logger {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.formatter = formatter
	return l
}

func (l *Logger) SetOutput(out io.Writer) *Logger {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.out = out
	return l
}

func (l *Logger) Start(msgLen ...int64) {
	if !atomic.CompareAndSwapUint32(&l.running, 0, 1) {
		return
	}

	var msgChanLen int64 = defaultAsyncMsgLen
	if len(msgLen) > 0 && msgLen[0] > 0 {
		msgChanLen = msgLen[0]
	}

	l.msgChan = make(chan *LogEvent, msgChanLen)
	l.signalChan = make(chan string, 1)
	l.wg.Add(1)

	worker := func() {
		isClose := false
		for {
			select {
			case event := <-l.msgChan:
				l.writeLog(event)
			case sg := <-l.signalChan:
				if sg == "close" {
					// Now should only send "flush" or "close" to bl.signalChan
					l.cleanChan()
					isClose = true
				}

				l.wg.Done()
			}

			if isClose {
				return
			}
		}
	}

	go worker()
}

func (l *Logger) Stop() {
	if !atomic.CompareAndSwapUint32(&l.running, 1, 0) {
		return
	}

	l.signalChan <- "close"
	l.wg.Wait()
	close(l.msgChan)
	close(l.signalChan)
}

func (l *Logger) Trace() *LogEvent {
	return l.newLogEvent(context.Background(), TraceLevel)
}

func (l *Logger) Debug() *LogEvent {
	return l.newLogEvent(context.Background(), DebugLevel)
}

func (l *Logger) Info() *LogEvent {
	return l.newLogEvent(context.Background(), InfoLevel)
}

func (l *Logger) Warn() *LogEvent {
	return l.newLogEvent(context.Background(), WarnLevel)
}

func (l *Logger) Error() *LogEvent {
	return l.newLogEvent(context.Background(), ErrorLevel)
}

func (l *Logger) CtxTrace(ctx context.Context) *LogEvent {
	return l.newLogEvent(ctx, TraceLevel)
}

func (l *Logger) CtxDebug(ctx context.Context) *LogEvent {
	return l.newLogEvent(ctx, DebugLevel)
}

func (l *Logger) CtxInfo(ctx context.Context) *LogEvent {
	return l.newLogEvent(ctx, InfoLevel)
}

func (l *Logger) CtxWarn(ctx context.Context) *LogEvent {
	return l.newLogEvent(ctx, WarnLevel)
}

func (l *Logger) CtxError(ctx context.Context) *LogEvent {
	return l.newLogEvent(ctx, ErrorLevel)
}

func (l *Logger) newLogEvent(ctx context.Context, level Level) *LogEvent {
	if Level(atomic.LoadUint32((*uint32)(&l.level))) >= level {
		event, ok := l.eventPool.Get().(*LogEvent)
		if ok {
			event.Level = level
			event.Data = make(Fields, 6)
			event.Context = ctx
			return event
		}

		return &LogEvent{
			logger:  l,
			Level:   level,
			Data:    make(Fields, 6),
			Context: ctx,
		}
	}
	return nil
}

func (l *Logger) getFileFlag() uint32 {
	return atomic.LoadUint32(&l.fileFlag)
}

func (l *Logger) log(event *LogEvent) {
	if atomic.LoadUint32(&l.running) != 1 {
		return
	}

	select {
	case l.msgChan <- event:
	default:
	}
}

func (l *Logger) cleanChan() {
	msgChanEmpty := false
	for {
		select {
		case event := <-l.msgChan:
			l.writeLog(event)
		default:
			msgChanEmpty = true
		}

		if msgChanEmpty {
			break
		}
	}
}

func (l *Logger) writeLog(event *LogEvent) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	err := l.hooks.Fire(event.Level, event)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to fire hook: %v\n", err)
	}

	serialized, err := l.formatter.Format(event)
	l.releaseLogEvent(event)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to obtain reader, %v\n", err)
		return
	}

	if _, err := l.out.Write(serialized); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write to log, %v\n", err)
	}
}

func (l *Logger) releaseLogEvent(event *LogEvent) {
	event.Data = map[string]interface{}{}
	event.Context = nil
	event.Err = nil
	l.eventPool.Put(event)
}
