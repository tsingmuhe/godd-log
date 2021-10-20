package logs

import (
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
)

const (
	Ldate         uint32 = 1 << iota // the date in the local time zone: 2009/01/23
	Ltime                            // the time in the local time zone: 01:23:23
	Lmicroseconds                    // microsecond resolution: 01:23:23.123123.  assumes Ltime.
	Llongfile                        // full file name and line number: /a/b/c/d.go:23
	Lshortfile                       // final file name element and line number: d.go:23. overrides Llongfile
)

const (
	defaultAsyncMsgLen = 1e3
)

type Logger struct {
	mu sync.RWMutex

	level     Level
	flag      uint32
	hooks     LevelHooks
	formatter Formatter
	out       io.Writer

	eventPool sync.Pool

	asynchronous uint32
	msgChan      chan *LogEvent
	signalChan   chan string
	wg           sync.WaitGroup
}

func New(level Level, formatter Formatter, out io.Writer, flag uint32) *Logger {
	return &Logger{
		level:     level,
		flag:      flag,
		formatter: formatter,
		out:       out,
		hooks:     make(LevelHooks),
	}
}

func (l *Logger) SetLevel(level Level) {
	atomic.StoreUint32((*uint32)(&l.level), uint32(level))
}

func (l *Logger) SetFlags(flag uint32) {
	atomic.StoreUint32(&l.flag, flag)
}

func (l *Logger) GetFlags() uint32 {
	return atomic.LoadUint32(&l.flag)
}

func (l *Logger) AddHook(hook Hook) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.hooks.Add(hook)
}

func (l *Logger) SetFormatter(formatter Formatter) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.formatter = formatter
}

func (l *Logger) SetOutput(out io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.out = out
}

func (l *Logger) Async(msgLen ...int64) {
	if !atomic.CompareAndSwapUint32(&l.asynchronous, 0, 1) {
		return
	}

	var msgChanLen int64 = defaultAsyncMsgLen
	if len(msgLen) > 0 && msgLen[0] > 0 {
		msgChanLen = msgLen[0]
	}

	l.msgChan = make(chan *LogEvent, msgChanLen)
	l.signalChan = make(chan string, 1)
	l.wg.Add(1)

	go l.startWorker()
}

func (l *Logger) startWorker() {
	isClose := false
	for {
		select {
		case event := <-l.msgChan:
			l.doLog(event)
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

func (l *Logger) cleanChan() {
	msgChanEmpty := false
	for {
		select {
		case event := <-l.msgChan:
			l.doLog(event)
		default:
			msgChanEmpty = true
		}

		if msgChanEmpty {
			break
		}
	}
}

func (l *Logger) StopAsync() {
	if !atomic.CompareAndSwapUint32(&l.asynchronous, 1, 0) {
		return
	}

	l.signalChan <- "close"
	l.wg.Wait()
	close(l.msgChan)
	close(l.signalChan)
}

func (l *Logger) Trace() *LogEvent {
	return l.newLogEvent(TraceLevel)
}

func (l *Logger) Debug() *LogEvent {
	return l.newLogEvent(DebugLevel)
}

func (l *Logger) Info() *LogEvent {
	return l.newLogEvent(InfoLevel)
}

func (l *Logger) Warn() *LogEvent {
	return l.newLogEvent(WarnLevel)
}

func (l *Logger) Error() *LogEvent {
	return l.newLogEvent(ErrorLevel)
}

func (l *Logger) Fatal() *LogEvent {
	return l.newLogEvent(FatalLevel)
}

func (l *Logger) Panic() *LogEvent {
	return l.newLogEvent(PanicLevel)
}

func (l *Logger) newLogEvent(level Level) *LogEvent {
	return NewLogEvent(l, level)
}

func (l *Logger) isLevelEnabled(level Level) bool {
	return Level(atomic.LoadUint32((*uint32)(&l.level))) >= level
}

func (l *Logger) log(event *LogEvent) {
	if atomic.LoadUint32(&l.asynchronous) == 1 {
		select {
		case l.msgChan <- event:
		default:
			l.doLog(event)
		}
	} else {
		l.doLog(event)
	}
}

func (l *Logger) doLog(event *LogEvent) {
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
	l.eventPool.Put(event)
}
