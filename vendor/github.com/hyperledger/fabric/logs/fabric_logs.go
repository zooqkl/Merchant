package logs

import (
	"encoding/json"
	"fmt"
	beegoLogs "github.com/astaxie/beego/logs"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/spf13/viper"
)

// Log levels to control the logging output.
const (
	LevelEmergency = iota
	LevelAlert
	LevelCritical
	LevelError
	LevelWarning
	LevelNotice
	LevelInformational
	LevelDebug
)

type FabricLogger struct {
	logger *beegoLogs.BeeLogger
}

type yxLogInfo struct {
	isOpenYxlog     bool
	filepath        string
	filename        string
	maxlinesPerFile int
	maxsizePerFile  int
	maxTotalSize    int64
	isAutoDelete    bool
	daily           bool
	rotate          bool
	maxdays         int
}

const (
	Orderer_Prefix = "ORDERER"
	Peer_Prefix    = "CORE"
	FilenameLenMax = 128
	FilepathLenMax = 128
)

var once sync.Once
var fl *FabricLogger
var loglevel int = LevelDebug
var levelNames = [...]string{"emergency", "alert", "critical", "error", "warning", "notice", "info", "debug"}

func GetFabricLogger(is_orderer ...interface{}) *FabricLogger {
	once.Do(func() {
		var fabricCfgPath = os.Getenv("FABRIC_CFG_PATH")
		var configName string
		var loginfo yxLogInfo
		var loglevelStr string

		_, err := os.Stat(fabricCfgPath)
		if err == nil {
			if len(is_orderer) == 0 {
				//非orderer节点
				configName = strings.ToLower(Peer_Prefix)
				config := viper.New()
				config.SetConfigName(configName)
				config.AddConfigPath(fabricCfgPath)
				config.ReadInConfig()
				config.SetEnvPrefix("CORE")
				config.AutomaticEnv()
				replacer := strings.NewReplacer(".", "_")
				config.SetEnvKeyReplacer(replacer)
				config.SetConfigType("yaml")

				loginfo.isOpenYxlog = config.GetBool("logging.isOpenYxlog")
				loginfo.filepath = config.GetString("logging.logpath")
				loginfo.filename = config.GetString("logging.logname")
				loginfo.maxlinesPerFile = config.GetInt("logging.maxlinesPerFile")
				loginfo.maxsizePerFile = config.GetInt("logging.maxsizePerFile")
				loginfo.maxTotalSize, _ = strconv.ParseInt(config.GetString("logging.maxTotalSize"), 10, 64)
				loginfo.maxdays = config.GetInt("logging.maxdays")
				loginfo.daily = config.GetBool("logging.daily")
				loginfo.rotate = loginfo.isOpenYxlog
				loginfo.isAutoDelete = config.GetBool("logging.isautodelete")
				loglevelStr = config.GetString("logging.yxLogLevel")
				// fmt.Printf("peer loglevel:%d\n\n", loglevel)
			} else {
				configName = strings.ToLower(Orderer_Prefix)
				config := viper.New()
				config.SetConfigName(configName)
				config.AddConfigPath(fabricCfgPath)
				config.ReadInConfig()
				config.SetEnvPrefix("ORDERER")
				config.AutomaticEnv()
				replacer := strings.NewReplacer(".", "_")
				config.SetEnvKeyReplacer(replacer)
				config.SetConfigType("yaml")

				loginfo.isOpenYxlog = config.GetBool("general.isOpenYxlog")
				loginfo.filepath = config.GetString("general.logpath")
				loginfo.filename = config.GetString("general.logname")
				loginfo.maxlinesPerFile = config.GetInt("general.maxlinesPerFile")
				loginfo.maxsizePerFile = config.GetInt("general.maxsizePerFile")
				loginfo.maxTotalSize, _ = strconv.ParseInt(config.GetString("general.maxTotalSize"), 10, 64)
				loginfo.maxdays = config.GetInt("general.maxdays")
				loginfo.daily = config.GetBool("general.daily")
				loginfo.rotate = loginfo.isOpenYxlog
				loginfo.isAutoDelete = config.GetBool("general.isautodelete")
				loglevelStr = config.GetString("general.yxLogLevel")
				// fmt.Printf("orderer loglevel:%d\n\n", loglevel)
			}
		} else {
			loglevelStr = os.Getenv("CHAINCODE_LOG_LEVEL")
			loginfo.isOpenYxlog, _ = strconv.ParseBool(os.Getenv("CHAINCODE_LOG_ISOPENYXLOG"))
			loginfo.filepath = os.Getenv("CHAINCODE_LOG_DESTINATION")
			loginfo.maxlinesPerFile, _ = strconv.Atoi(os.Getenv("CHAINCODE_LOG_MAXLINES"))
			loginfo.maxsizePerFile, _ = strconv.Atoi(os.Getenv("CHAINCODE_LOG_MAXSIZE"))
			loginfo.maxTotalSize, _ = strconv.ParseInt(os.Getenv("CHAINCODE_LOG_MAXTOTALSIZE"), 10, 64)
			loginfo.isAutoDelete, _ = strconv.ParseBool(os.Getenv("CHAINCODE_LOG_ISAUTODELETE"))
			loginfo.daily, _ = strconv.ParseBool(os.Getenv("CHAINCODE_LOG_DAILY"))
			loginfo.rotate = loginfo.isOpenYxlog
			loginfo.maxdays, _ = strconv.Atoi(os.Getenv("CHAINCODE_LOG_MAXDAYS"))
			loginfo.filename = "cc_log"
		}

		if loginfo.filepath == "" || len(loginfo.filepath) > FilenameLenMax {
			fmt.Printf("log config args err,filepath:%s,filepath_len:%d,use default arg.\n", loginfo.filepath, len(loginfo.filepath))
			loginfo.filepath = "/var/fabric_logs"
		}

		if loginfo.filename == "" || len(loginfo.filename) > FilepathLenMax {
			fmt.Printf("log config args err,filename:%s,filename_len:%d,use default arg.\n", loginfo.filename, len(loginfo.filename))
			loginfo.filename = "yx_log"
		}

		if loginfo.maxlinesPerFile <= 0 || loginfo.maxlinesPerFile > math.MaxInt32 {
			fmt.Printf("log config args err,maxlinesPerFile:%d,use default arg.\n", loginfo.maxlinesPerFile)
			loginfo.maxlinesPerFile = 10000000
		}

		if loginfo.maxsizePerFile <= 0 || loginfo.maxsizePerFile > math.MaxInt32 {
			fmt.Printf("log config args err,maxsizePerFile:%d,use default arg.\n", loginfo.maxsizePerFile)
			loginfo.maxsizePerFile = 102400000
		}

		if loginfo.maxTotalSize <= 0 || loginfo.maxTotalSize > math.MaxInt64 {
			fmt.Printf("log config args err,maxTotalSize:%d,use default arg.\n", loginfo.maxTotalSize)
			loginfo.maxTotalSize = 4096000000
		}

		if loginfo.maxdays <= 0 || loginfo.maxdays > math.MaxInt32 {
			fmt.Printf("log config args err,maxdays:%d,use default arg.\n", loginfo.maxdays)
			loginfo.maxdays = 15
		}

		if loglevelStr == "" {
			fmt.Printf("log config args err,loglevel:%s,use default arg.\n", loglevelStr)
			loglevel = LevelInformational
		} else {
			i, err := strconv.Atoi(loglevelStr)
			if err != nil || i < 0 || i > 7 {
				fmt.Printf("log config args err,loglevel:%s,use default arg.\n", loglevelStr)
				loglevel = LevelInformational
			} else {
				loglevel = i
			}
		}

		// fmt.Printf("filepath:%s, fileName:%s, maxlinesPerFile:%d, maxsizePerFile:%d, maxdays:%d, daily:%t, rotate:%t, maxTotalSize:%d, isdelete:%t, loglevel:%d\n\n\n\n\n", loginfo.filepath, loginfo.filename, loginfo.maxlinesPerFile, loginfo.maxsizePerFile, loginfo.maxdays, loginfo.daily, loginfo.rotate, loginfo.maxTotalSize, loginfo.isAutoDelete, loglevel)

		fl = GetLogger(loginfo)

	})
	return fl
}

func GetLogger(loginfo yxLogInfo) *FabricLogger {
	var separateFile []string

	if loginfo.isOpenYxlog == false {
		fmt.Println("use default log.")
		return nil
	}

	os.MkdirAll(loginfo.filepath, 0755)

	l := beegoLogs.NewLogger(10000)
	l.EnableFuncCallDepth(true)

	for i := LevelEmergency; i <= loglevel; i++ {
		separateFile = append(separateFile, levelNames[i])
	}
	separateFileJson, _ := json.Marshal(separateFile)
	separate := fmt.Sprintf(`"separate":%s`, separateFileJson)

	config := fmt.Sprintf(`"filename":"%s/%s", "maxlines":%d, "maxsize":%d, "maxtotalsize":%d, "daily": %t, "rotate": %t, "maxdays": %d, "isautodelete":%t, `, loginfo.filepath, loginfo.filename, loginfo.maxlinesPerFile, loginfo.maxsizePerFile, loginfo.maxTotalSize, loginfo.daily, loginfo.rotate, loginfo.maxdays, loginfo.isAutoDelete)
	config = "{" + config + separate + "}"

	l.SetLogger(beegoLogs.AdapterMultiFile, config)

	fabricLogger := &FabricLogger{
		logger: l,
	}
	return fabricLogger
}

func (l *FabricLogger) Debug(v ...interface{}) {
	if loglevel < LevelDebug {
		return
	}
	l.logger.Debug("", v...)
}

func (l *FabricLogger) Debugf(formatString string, v ...interface{}) {
	if loglevel < LevelDebug {
		return
	}
	l.logger.Debug(formatString, v...)
}

func (l *FabricLogger) Info(v ...interface{}) {
	if loglevel < LevelInformational {
		return
	}
	l.logger.Info("", v...)
}

func (l *FabricLogger) Infof(formatString string, v ...interface{}) {
	if loglevel < LevelInformational {
		return
	}
	l.logger.Info(formatString, v...)
}

func (l *FabricLogger) Notice(v ...interface{}) {
	if loglevel < LevelNotice {
		return
	}
	l.logger.Notice("", v...)
}

func (l *FabricLogger) Noticef(formatString string, v ...interface{}) {
	if loglevel < LevelNotice {
		return
	}
	l.logger.Notice(formatString, v...)
}

func (l *FabricLogger) Warning(v ...interface{}) {
	if loglevel < LevelWarning {
		return
	}
	l.logger.Warning("", v...)
}

func (l *FabricLogger) Warningf(formatString string, v ...interface{}) {
	if loglevel < LevelWarning {
		return
	}
	l.logger.Warning(formatString, v...)
}

func (l *FabricLogger) Error(v ...interface{}) {
	if loglevel < LevelError {
		return
	}
	l.logger.Error("", v...)
}

func (l *FabricLogger) Errorf(formatString string, v ...interface{}) {
	if loglevel < LevelError {
		return
	}
	l.logger.Error(formatString, v...)
}

func (l *FabricLogger) Critical(v ...interface{}) {
	if loglevel < LevelCritical {
		return
	}
	l.logger.Critical("", v...)
}

func (l *FabricLogger) Criticalf(formatString string, v ...interface{}) {
	if loglevel < LevelCritical {
		return
	}
	l.logger.Critical(formatString, v...)
}

func (l *FabricLogger) Alert(v ...interface{}) {
	if loglevel < LevelAlert {
		return
	}
	l.logger.Alert("", v...)
}

func (l *FabricLogger) Alertf(formatString string, v ...interface{}) {
	if loglevel < LevelAlert {
		return
	}
	l.logger.Alert(formatString, v...)
}

func (l *FabricLogger) Emergency(v ...interface{}) {
	if loglevel < LevelEmergency {
		return
	}
	l.logger.Emergency("", v...)
}

func (l *FabricLogger) Emergencyf(formatString string, v ...interface{}) {
	if loglevel < LevelEmergency {
		return
	}
	l.logger.Emergency(formatString, v...)
}
