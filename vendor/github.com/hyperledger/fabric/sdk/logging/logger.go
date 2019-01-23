/*
Copyright Yunphant Corp. All Rights Reserved.
*/

package sdklogging

import (
	"github.com/op/go-logging"
	"io"
	"os"
	"fmt"
	"time"
)

const (
	pkgLogID      = "sdklogging"
	defaultFormat = "%{color}%{time:2006-01-02 15:04:05.000 MST} [%{shortfile}] [%{module}] %{shortfunc} -> %{level:.4s} %{id:03x} %{message}%{color:reset}"
	defaultLevel  = logging.ERROR
)

var (
	loggerInstance *LoggerImpl
	InstanceMap = map[string]*LoggerImpl{}
	FileMap = map[string]*os.File{}
)

// set args to get that logger, otherwise is sdklogging
func GetLogger() Logger {
	name := pkgLogID
	_,ok := InstanceMap[name]

	if !ok {
		InitLogger("DEBUG","DEBUG")
		loggerInstance.logger.Debug("Init logger with DEBUG level before sdk call InitLogger method and set level to config level.")
	}
	InstanceMap[name] = loggerInstance
	return loggerInstance
}

// SetFormat sets the logging format.
func setFormat(formatSpec string) logging.Formatter {
	if formatSpec == "" {
		formatSpec = defaultFormat
	}
	return logging.MustStringFormatter(formatSpec)
}

func getLogFile() *os.File {
	today := time.Now().Format("2006-01-02")
	//today := time.Now().String()
	logFileName := "logs/"+today+".txt"
	defaultOutput,_ := os.OpenFile(logFileName, os.O_CREATE|os.O_APPEND|os.O_RDWR, os.ModePerm|os.ModeTemporary)
	return defaultOutput
}


func InitLogger(sdkLevel, fabricLevel string) Logger {

	name := pkgLogID
	_,ok := InstanceMap[name]
	if !ok {
		loggerInstance = &LoggerImpl{
			logger: logging.MustGetLogger(name),
		}
		// logger is wrapped in loggerInstance,set ExtraCalldepth=1 to print calldepth where logger.XXX() is called
		loggerInstance.logger.ExtraCalldepth = 1
	}

	var sdklevel logging.Level
	var fabriclevel logging.Level
	var err error

	sdklevel, err = logging.LogLevel(sdkLevel)
	if err != nil {
		loggerInstance.Errorf("[GetLogger] Get sdk log level from config error,set level to ERROR: %s", err)
	}
	fabriclevel, err = logging.LogLevel(fabricLevel)
	if err != nil {
		loggerInstance.Errorf("[GetLogger] Get fabric log level from config error,set level to ERROR: %s", err)
	}

//<<<<<<< HEAD
//	fileoutput := initBackend(setFormat(defaultFormat), sdklevel, fabriclevel)
//	FileMap[name] = fileoutput
//	fmt.Printf("loggerinstance is %v\n",loggerInstance)
//
//=======
	initBackend(setFormat(defaultFormat), sdklevel, fabriclevel)
//>>>>>>> fe24c60... [#1557] feat: add dapp example
	return loggerInstance
}

//func getLogFile() *os.File {
//	today := time.Now().Format("2006-01-02")
//	//today := time.Now().String()
//	logFileName := "logs/"+today+".txt"
//	defaultOutput,_ := os.OpenFile(logFileName, os.O_CREATE|os.O_APPEND|os.O_RDWR, os.ModePerm|os.ModeTemporary)
//	return defaultOutput
//}


// InitBackend sets up the logging backend based on
// the provided logging formatter and I/O writer.
func initBackend(formatter logging.Formatter, sdkLevel, fabricLevel logging.Level) *os.File {
	// get file to save log
	fileOutput := getLogFile()

	writers := []io.Writer{
		os.Stdout,
		fileOutput,
	}
	multiwriters := io.MultiWriter(writers...)
	backend := logging.NewLogBackend(multiwriters, "", 0)
	backendFormatter := logging.NewBackendFormatter(backend, formatter)

	// set default log level to high level
	// logging.SetBackend(backendFormatter).SetLevel(logging.WARNING, "")
	logging.SetBackend(backendFormatter).SetLevel(fabricLevel, "")
	// and set sdk log level
	logging.SetLevel(logging.DEBUG, "")
	logging.SetLevel(sdkLevel, pkgLogID)
	return fileOutput
}


//<<<<<<< HEAD
//=======

//>>>>>>> fe24c60... [#1557] feat: add dapp example
// change logger at 0 a.m.
func Changelogger() {
	now := time.Now()
	//next := now.Add(time.Hour * 24)
	//next = time.Date(next.Year(), next.Month(), next.Day(), 0, 0, 0, 0, next.Location())
	next := now.Add(time.Hour * 24)
	next = time.Date(next.Year(), next.Month(), next.Day(), 0, 0, 0, 0, next.Location())

	t := time.NewTimer(next.Sub(now))
	for {
		select {
		case <-t.C:
			fmt.Printf("create a new logger at: %v\n",time.Now())
			FileMap[pkgLogID].Close()
			InitBackendDaily()
			//t.Reset(time.Hour*24)
			t.Reset(time.Hour*24)

		}
	}
}


func InitBackendDaily()  {
	name := pkgLogID
	loggerInstance = &LoggerImpl{
		logger: logging.MustGetLogger(name),
	}
	// logger is wrapped in loggerInstance,set ExtraCalldepth=1 to print calldepth where logger.XXX() is called
	loggerInstance.logger.ExtraCalldepth = 1

	var sdklevel logging.Level
	var fabriclevel logging.Level
	var err error

	sdklevel, err = logging.LogLevel("debug")
	if err != nil {
		loggerInstance.Errorf("[GetLogger] Get sdk log level from config error,set level to ERROR: %s", err)
	}
	fabriclevel, err = logging.LogLevel("debug")
	if err != nil {
		loggerInstance.Errorf("[GetLogger] Get fabric log level from config error,set level to ERROR: %s", err)
	}

	fileoutput := initBackend(setFormat(defaultFormat), sdklevel, fabriclevel)
	InstanceMap[name] = loggerInstance
	FileMap[name] = fileoutput

}


//<<<<<<< HEAD
//=======


//>>>>>>> fe24c60... [#1557] feat: add dapp example
type LoggerImpl struct {
	logger *logging.Logger
}

func (l *LoggerImpl) Debug(args ...interface{}) {
	l.logger.Debug(args...)
}

func (l *LoggerImpl) Debugf(format string, args ...interface{}) {
	l.logger.Debugf(format, args...)
}

func (l *LoggerImpl) Info(args ...interface{}) {
	l.logger.Info(args...)
}

func (l *LoggerImpl) Infof(format string, args ...interface{}) {
	l.logger.Infof(format, args...)
}

func (l *LoggerImpl) Notice(args ...interface{}) {
	l.logger.Notice(args...)
}

func (l *LoggerImpl) Noticef(format string, args ...interface{}) {
	l.logger.Noticef(format, args...)
}

func (l *LoggerImpl) Warn(args ...interface{}) {
	l.logger.Warning(args...)
}

func (l *LoggerImpl) Warnf(format string, args ...interface{}) {
	l.logger.Warningf(format, args...)
}

func (l *LoggerImpl) Error(args ...interface{}) {
	l.logger.Error(args...)
}

func (l *LoggerImpl) Errorf(format string, args ...interface{}) {
	l.logger.Errorf(format, args...)
}

func (l *LoggerImpl) Fatal(args ...interface{}) {
	l.logger.Fatal(args...)
}

func (l *LoggerImpl) Fatalf(format string, args ...interface{}) {
	l.logger.Fatalf(format, args...)
}
