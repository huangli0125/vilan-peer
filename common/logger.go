package common

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

type LogType byte

const (
	Error LogType = 0
	Warn  LogType = 1
	Info  LogType = 2
	Debug LogType = 3
)

type OnLog func(LogType, string, string)

type Logger struct {
	enable bool
	save   bool
	filter LogType
	file   *os.File
	path   string
	date   string
	logger *log.Logger
	onLog  OnLog
	mutex  *sync.Mutex
}

func NewLogger(enable bool, save bool, filter LogType) *Logger {
	if filter > Debug || filter < Error {
		panic("filter invalid")
	}
	return &Logger{enable: enable, save: save, filter: filter, mutex: &sync.Mutex{}}
}
func (l *Logger) Enable() {
	l.enable = true
}
func (l *Logger) Disable() {
	l.enable = false
}

func (l *Logger) SetLogLevel(level LogType) {
	l.filter = level
}

func (l *Logger) SetFunc(onLog OnLog) {
	l.onLog = onLog
}
func (l *Logger) Debug(msg ...interface{}) {
	if !l.enable {
		return
	}
	if byte(l.filter) < byte(Debug) {
		return
	}
	l.mutex.Lock()
	defer l.mutex.Unlock()
	if l.save {
		l.saveToFile(Debug, msg)
	}
	pc, file, line, _ := runtime.Caller(1) // 0 -3 四级调用层级
	pcName := runtime.FuncForPC(pc).Name() // 获取方法名称
	temp := strings.Split(file, "/")
	file = temp[len(temp)-1]
	temp = strings.Split(pcName, ".")
	method := temp[len(temp)-1]
	t := time.Now().Format("15:04:05.000")
	m := fmt.Sprint(msg)
	fmt.Println("Debug—>", t, "[", file, ",", method, ",", line, ",]", "—>", m)
	if l.onLog != nil {
		l.onLog(Debug, t, m)
	}
}
func (l *Logger) Info(msg ...interface{}) {
	if !l.enable {
		return
	}
	if byte(l.filter) < byte(Info) {
		return
	}
	l.mutex.Lock()
	defer l.mutex.Unlock()
	if l.save {
		l.saveToFile(Info, msg)
	}

	pc, file, line, _ := runtime.Caller(1) // 0 -3 四级调用层级
	pcName := runtime.FuncForPC(pc).Name() // 获取方法名称
	temp := strings.Split(file, "/")
	file = temp[len(temp)-1]
	temp = strings.Split(pcName, ".")
	method := temp[len(temp)-1]
	t := time.Now().Format("15:04:05.000")
	m := fmt.Sprint(msg)
	fmt.Println("Info—>", t, "[", file, ",", method, ",", line, ",]", "—>", m)
	//fmt.Println("Info——>", time.Now().Format("15:04:05.000"), "—>", message)
	if l.onLog != nil {
		l.onLog(Info, t, m)
	}
}
func (l *Logger) Warn(msg ...interface{}) {
	if !l.enable {
		return
	}
	if byte(l.filter) < byte(Warn) {
		return
	}
	l.mutex.Lock()
	defer l.mutex.Unlock()
	if l.save {
		l.saveToFile(Warn, msg)
	}
	pc, file, line, _ := runtime.Caller(1) // 0 -3 四级调用层级
	pcName := runtime.FuncForPC(pc).Name() // 获取方法名称
	temp := strings.Split(file, "/")
	file = temp[len(temp)-1]
	temp = strings.Split(pcName, ".")
	method := temp[len(temp)-1]
	t := time.Now().Format("15:04:05.000")
	m := fmt.Sprint(msg)
	fmt.Println("Warn—>", t, "[", file, ",", method, ",", line, ",]", "—>", m)
	if l.onLog != nil {
		l.onLog(Warn, t, m)
	}
}
func (l *Logger) Error(msg ...interface{}) {
	if !l.enable {
		return
	}
	if byte(l.filter) < byte(Error) {
		return
	}
	l.mutex.Lock()
	defer l.mutex.Unlock()
	if l.save {
		l.saveToFile(Error, msg)
	}
	pc, file, line, _ := runtime.Caller(1) // 0 -3 四级调用层级
	pcName := runtime.FuncForPC(pc).Name() // 获取方法名称
	temp := strings.Split(file, "/")
	file = temp[len(temp)-1]
	temp = strings.Split(pcName, ".")
	method := temp[len(temp)-1]
	t := time.Now().Format("15:04:05.000")
	m := fmt.Sprint(msg)
	fmt.Println("Error—>", t, "[", file, ",", method, ",", line, ",]", "—>", m)
	//fmt.Println("Error——>", time.Now().Format("15:04:05.000"), "—>", message)
	if l.onLog != nil {
		l.onLog(Error, t, m)
	}
}

func (l *Logger) saveToFile(logType LogType, msg ...interface{}) {
	defer func() {
		if err := recover(); err != nil {
			fmt.Println("日志组件出错:", err)
		}
	}()
	d := time.Now().Format("20060102")
	if l.date != d {
		if l.file != nil {
			_ = l.file.Close()
			l.file = nil
			l.logger = nil
		}
		l.date = d
		l.path = GetLogPath() + l.date + ".log"
		f, err := os.OpenFile(l.path, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0444)
		if err == nil {
			l.file = f
			l.logger = log.New(f, "", log.Ltime|log.Lmicroseconds)
		}
	}
	if l.logger != nil {
		if byte(l.filter) < byte(logType) {
			return
		}
		pc, file, line, _ := runtime.Caller(2) // 0 -3 四级调用层级
		pcName := runtime.FuncForPC(pc).Name() // 获取方法名称
		temp := strings.Split(file, "/")
		file = temp[len(temp)-1]
		temp = strings.Split(pcName, ".")
		method := temp[len(temp)-1]
		switch logType {
		case Debug:
			l.logger.SetPrefix(fmt.Sprintf("Debug—>%s [%s,%s,%d]—>", time.Now().Format("15:04:05.000"), file, method, line))
		case Info:
			l.logger.SetPrefix(fmt.Sprintf("Info—>%s [%s,%s,%d]—>", time.Now().Format("15:04:05.000"), file, method, line))
		case Warn:
			l.logger.SetPrefix(fmt.Sprintf("Warn—>%s [%s,%s,%d]—>", time.Now().Format("15:04:05.000"), file, method, line))
		case Error:
			l.logger.SetPrefix(fmt.Sprintf("Error—>%s [%s,%s,%d]—>", time.Now().Format("15:04:05.000"), file, method, line))
		}
		l.logger.Println(msg)
	}
}
