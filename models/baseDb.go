package models

import (
	_ "github.com/jinzhu/gorm/dialects/postgres"
	"fmt"
	"Merchant/logging"
	"github.com/jinzhu/gorm"
	"time"
)

var logger merchantlogging.Logger
var dbConfig *PsqlInfo
var Dconnect *gorm.DB

func Init() {
	logger = merchantlogging.GetLogger()
}

type PsqlInfo struct {
	Host     string
	Port     uint16
	User     string
	Password string
	DbName   string
	DbType   string
}

func PreDbTask(psqConfig *PsqlInfo) {
	Init()
	dbConfig = psqConfig
	db, err := GetDbConn()
	if err != nil {
		logger.Fatalf("Error creating db connection: %s", err)
	}
	//mike 为0表示不限制
	db.DB().SetMaxOpenConns(10)
	db.DB().SetMaxIdleConns(0)
	db.DB().SetConnMaxLifetime(10 * time.Minute)
	if err := db.Debug().Exec("set transaction isolation level serializable").AutoMigrate(
		&UserTable{},
		&TicketTable{}).Error; err != nil {
		logger.Fatal("Error auto-migrating database : %s", err)
	}
}

// create db connection
func GetDbConn() (*gorm.DB, error) {
	if Dconnect == nil {
		psqlInfo := fmt.Sprintf("host=%s port=%d dbname=%s user=%s password=%s  sslmode=disable ", dbConfig.Host, dbConfig.Port, dbConfig.DbName, dbConfig.User, dbConfig.Password)
		logger.Debug(psqlInfo)
		engine, err := gorm.Open(dbConfig.DbType, psqlInfo)
		if err != nil {
			logger.Fatal("Connect %s fail! Error is %s", dbConfig.DbType, err)
		}
		Dconnect = engine
		logger.Debugf("connect postgresql %s success", dbConfig.DbName)
	}
	return Dconnect, nil
}
