package main

import (
	"Merchant/logging"
	"github.com/gin-gonic/gin"
	"Merchant/models"
	"fmt"
)

var logger merchantlogging.Logger

func init() {
	logger = merchantlogging.GetLogger()
}

func main() {
	config := NewConfigFromYaml("../config/app.yaml")
	logger.Debug(config)
	dbConfig := &models.PsqlInfo{config.DbIp, config.DbPort, config.DbUser, config.DbPasswd, config.DbName, config.DbType}
	models.PreDbTask(dbConfig)
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())

	router.Use(func(context *gin.Context) {
		context.Set("app_config", config)
		context.Next()
	})

	router.POST("/login", Login)
	router.POST("/register", Register)
	router.POST("/change-password", ChangePassword)

	authorize := router.Group("/ticket", JWTAuth())
	{
		authorize.POST("/query", TicketQuery)
		authorize.POST("/consume", TicketConsume)
	}
	addr := fmt.Sprintf(":%d", config.HttpPort)
	router.Run(addr)

}
