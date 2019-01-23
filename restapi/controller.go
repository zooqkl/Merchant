package main

import (
	"github.com/gin-gonic/gin"
	"Merchant/models"
	"fmt"
	"net/http"
	"github.com/dgrijalva/jwt-go"
	"time"
	"encoding/json"
)

func Login(c *gin.Context) {
	lr := new(LoginRequest)
	if err := c.ShouldBindJSON(lr); err == nil {
		if !(CheckEmail(lr.Name) || CheckPhone(lr.Name)) {
			logger.Debugf("name format Error,Expecting Email or phone ! name is %s", lr.Name)
			c.JSON(http.StatusOK, Http_error(models.FormatError, models.UsernameFormatError, nil))
			return
		}
		if !CheckPasswd(lr.Password) {
			logger.Debugf("password format Error!")
			c.JSON(http.StatusOK, Http_error(models.FormatError, models.PasswdFormatError, nil))
			return
		}

		db, _ := models.GetDbConn()

		dbuser := new(models.UserTable)
		//判断用户名是否存在
		if db.Where("username = ?", lr.Name).First(dbuser).RecordNotFound() {
			logger.Debugf("Login fail, please check username [%s]!", lr.Name)
			c.JSON(http.StatusOK, Http_error(models.UserIsNotExits, models.UserNameOrPasswdError, nil))
			return
		}
		logger.Debugf("dbuser is %v", dbuser)
		if lr.Name == dbuser.Username && HashPasswd(lr.Password) == dbuser.Password {
			//更新lastLoginTime
			if err := db.Model(dbuser).Update(models.UserTable{LastLoginTime: time.Now().String()}).Error; err != nil {
				logger.Errorf("Update lastLoginTime Fail error is %s!", err.Error())
				c.JSON(http.StatusOK, Http_error(models.ServerInternalErr, models.ServerInternalError, nil))
				return
			}
			config, ok := c.MustGet("app_config").(*Config)
			if !ok {
				c.JSON(http.StatusOK, Http_error(http.StatusInternalServerError, "Get config error.", nil))
				return
			}
			SetSignKey(config.SignKey)
			j := NewJWT()
			claims := CustomClaims{
				UserID:        dbuser.ID,
				UserName:      dbuser.Username,
				MerchantName:  dbuser.MerchantName,
				MerchantPhone: dbuser.MerchantPhone,
				StandardClaims: jwt.StandardClaims{
					NotBefore: int64(time.Now().Unix()),        // 签名生效时间 即时生效
					ExpiresAt: int64(time.Now().Unix() + 3600), // 过期时间  一小时
					Issuer:    config.SignKey,                  //签名的发行者
				},
			}

			token, err := j.CreateToken(claims)
			if err != nil {
				c.JSON(http.StatusOK, Http_error(models.UserLoginFail, fmt.Sprintf("CreateToken fail,Error is %s", err), nil))
				return
			}

			logger.Debugf("User %s Login success!", dbuser.Username)
			c.JSON(http.StatusOK, Http_success(models.UserLoginSuccess, fmt.Sprintf("User %s Login success!", dbuser.Username), token))
			return
		} else {
			c.JSON(http.StatusOK, Http_error(models.UserLoginFail, models.UserNameOrPasswdError, nil))
		}
	} else {
		c.JSON(http.StatusOK, Http_error(models.UserLoginFail, models.UserNameOrPasswdError, nil))
	}
}

func Register(c *gin.Context) {
	rr := new(RegisterRequest)
	if err := c.ShouldBindJSON(rr); err == nil {
		if !(CheckEmail(rr.Name) || CheckPhone(rr.Name)) {
			logger.Debug(models.UsernameFormatError)
			c.JSON(http.StatusOK, Http_error(models.FormatError, models.UsernameFormatError, nil))
			return
		}
		if !CheckPasswd(rr.Password) {
			logger.Debug(models.PasswdFormatError)
			c.JSON(http.StatusOK, Http_error(models.FormatError, models.PasswdFormatError, nil))
			return
		}
		if !CheckName(rr.MerchantName) {
			logger.Debugf("merchant_name format Error!")
			c.JSON(http.StatusOK, Http_error(models.FormatError, fmt.Errorf("merchant_name format Error!").Error(), nil))
			return
		}
		if !CheckPhone(rr.MerchantPhone) {
			logger.Debugf("merchant_phone format Error!")
			c.JSON(http.StatusOK, Http_error(models.FormatError, fmt.Errorf("merchant_phone format Error!").Error(), nil))
			return
		}

		db, _ := models.GetDbConn()

		// 判断数据库中是否有该用户
		if !db.Where("username = ?", rr.Name).First(new(models.UserTable)).RecordNotFound() {
			logger.Debug(rr.Name + models.UserIsExitsError)
			c.JSON(http.StatusOK, Http_error(models.UserIsExits, models.UserIsExitsError, nil))
			return
		}
		// 插入记录
		newUser := &models.UserTable{Username: rr.Name, Password: HashPasswd(rr.Password), MerchantName: rr.MerchantName, MerchantAddress: rr.MerchantAddress, MerchantPhone: rr.MerchantPhone, IsAvailable: models.AVAILABLE}
		if err := db.Create(newUser).Error; err != nil {
			logger.Errorf("Create user [%s] fail,Error is %s !", newUser.Username, err)
			c.JSON(http.StatusOK, Http_error(models.CreateUserFail, fmt.Errorf("Create user [%s] fail,Error is %s", newUser.Username, err).Error(), nil))
			return
		}
		logger.Debugf("Create user [%s] success!", newUser.Username)
		c.JSON(http.StatusOK, Http_success(models.CreateUserSuccess, fmt.Sprintf("Create user [%s] success!", newUser.Username), nil))
	} else {
		c.JSON(http.StatusOK, Http_error(models.CreateUserFail, err.Error(), nil))
	}
}

func ChangePassword(c *gin.Context) {
	cpr := new(ChangePasswdRequest)
	if err := c.ShouldBindJSON(cpr); err == nil {
		if !(CheckEmail(cpr.Name) || CheckPhone(cpr.Name)) {
			logger.Debugf("Name format Error,Expecting Email or phone !")
			c.JSON(http.StatusOK, Http_error(models.FormatError, models.UsernameFormatError, nil))
			return
		}
		if cpr.NewPassword != cpr.ConfirmPassword {
			c.JSON(http.StatusOK, Http_error(models.ChangePasswdFail, "Confirm password is not the same as  New password!", nil))
			return
		}
		if !CheckPasswd(cpr.OldPassword) || !CheckPasswd(cpr.NewPassword) || !CheckPasswd(cpr.ConfirmPassword) {
			logger.Debugf("Password format Error!")
			c.JSON(http.StatusOK, Http_error(models.ChangePasswdFail, models.PasswdFormatError, nil))
			return
		}

		db, _ := models.GetDbConn()

		dbuser := new(models.UserTable)
		//判断用户名是否存在
		if db.Where("username = ?", cpr.Name).First(dbuser).RecordNotFound() {
			logger.Debugf("Username %s is not Exits!", cpr.Name)
			c.JSON(http.StatusOK, Http_error(models.UserIsNotExits, models.UserNameOrPasswdError, nil))
			return
		}

		if dbuser.Password != HashPasswd(cpr.OldPassword) {
			logger.Debugf("Password verify fail!")
			c.JSON(http.StatusOK, Http_error(models.ChangePasswdFail, models.PasswdError, nil))
			return
		}

		if err := db.Model(dbuser).Update(models.UserTable{Password: HashPasswd(cpr.NewPassword)}).Error; err != nil {
			logger.Errorf("Change user [%s] password fail! err is %s", dbuser.Username, err.Error())
			c.JSON(http.StatusInternalServerError, Http_error(models.ServerInternalErr, models.ServerInternalError, nil))
			return
		}
		c.JSON(http.StatusOK, Http_success(models.ChangePasswdSuccess, fmt.Sprintf("Change user [%s]  password success!", dbuser.Username), nil))
	} else {
		c.JSON(http.StatusOK, Http_error(models.ChangePasswdFail, err.Error(), nil))
	}
}

func TicketQuery(c *gin.Context) {
	claims := c.MustGet("claims").(*CustomClaims)
	if claims == nil {
		c.JSON(http.StatusOK, Http_error(models.QueryTicketFail, "Token is invaild!", nil))
		return
	}
	logger.Debugf("%s call TicketQuery!", claims.MerchantName)
	tqr := new(TicketQueryRequest)
	if err := c.ShouldBindJSON(tqr); err == nil {
		spec := models.TicketInvokeSpec{
			TicketFcName: models.TicketQueryFcName,
			Args:         []string{tqr.TicketNumber},
		}
		result, err := models.Query(spec)
		if err != nil || nil == result {
			logger.Errorf("TicketQuery fail! error is %s", err.Error())
			c.JSON(http.StatusInternalServerError, Http_error(models.ServerInternalErr, fmt.Sprintf("TicketQuery fail! error is %s", err.Error()), nil))
			return
		}
		// fabric返回的数据，处理后返回给前端
		ticket := new(models.Ticket)
		err = json.Unmarshal(result, ticket)
		if err != nil {
			logger.Errorf("TicketQuery fail! Unmarshal fabric return data fail!  error is %s", err.Error())
			c.JSON(http.StatusInternalServerError, Http_error(models.ServerInternalErr, fmt.Sprintf("TicketQuery fail! Unmarshal fabric return data fail! error is %s", err.Error()), nil))
			return
		}
		logger.Debugf("TicketQuery result is %v", ticket)
		c.JSON(http.StatusOK, Http_success(models.QueryTicketSuccess, fmt.Sprintf("ticket_status is %s", ticket.TicketStatus.String()), ticket.TicketStatus))
	} else {
		c.JSON(http.StatusOK, Http_error(models.QueryTicketFail, err.Error(), nil))
	}
}

func TicketConsume(c *gin.Context) {
	claims := c.MustGet("claims").(*CustomClaims)
	if claims == nil {
		c.JSON(http.StatusOK, Http_error(models.ConsumeTicketFail, "Token is invaild!", nil))
		return
	}
	merchantName := claims.MerchantName
	userID := fmt.Sprintf("%d", claims.UserID)
	userName := claims.UserName
	logger.Debugf("%s call TicketConsume!,userID is %d ,userName is %s", merchantName, userID, userName)

	tcr := new(TicketConsumeRequest)
	if err := c.ShouldBindJSON(tcr); err == nil {
		// 消费链上的兑换码
		spec := models.TicketInvokeSpec{
			TicketFcName: models.TicketConsumeFcName,
			Args:         []string{tcr.TicketNumber, merchantName, userID, userName},
		}
		result, err := models.Invoke(spec)
		if (result != nil && result.Status == "VALID") || err != nil {
			logger.Errorf("TicketConsume fail! error is %s,fabric  result is %v", err.Error(), result)
			c.JSON(http.StatusInternalServerError, Http_error(models.ServerInternalErr, fmt.Sprintf("TicketConsume fail! error is %s", err.Error()), nil))
			return
		}
		// 消费成功，链上数据更新成功，数据库插入一条消费记录
		db, _ := models.GetDbConn()
		// 判断该票券是否在数据库中存在
		ticketConsumeRecord := new(models.TicketTable)
		if !db.Where("ticket_number = ?", tcr.TicketNumber).First(ticketConsumeRecord).RecordNotFound() {
			if err := db.Model(ticketConsumeRecord).Update(models.TicketTable{TicketNumber: tcr.TicketNumber, TicketStatus: models.USED, TicketMerchantName: merchantName, TicketTime: time.Now()}).Error; err != nil {
				logger.Errorf("updte ticketConsumeRecord [%s] fail,Error is %s !", ticketConsumeRecord.TicketNumber, err)
				c.JSON(http.StatusOK, Http_error(models.ConsumeTicketFail, fmt.Sprintf("updte ticketConsumeRecord [%s] fail,Error is %s !", ticketConsumeRecord.TicketNumber, err.Error()), nil))
				return
			}
			logger.Debugf("updte ticketConsumeRecord [%s] success!", ticketConsumeRecord.TicketNumber)
			c.JSON(http.StatusOK, Http_success(models.ConsumeTicketSuccess, fmt.Sprintf("updte ticketConsumeRecord [%s] success!", ticketConsumeRecord.TicketNumber), nil))
			return
		}

		ticketConsumeRecord = &models.TicketTable{TicketNumber: tcr.TicketNumber, TicketStatus: models.USED, TicketMerchantName: merchantName, TicketTime: time.Now()}
		if err := db.Create(ticketConsumeRecord).Error; err != nil {
			logger.Errorf("insert ticketConsumeRecord [%s] fail,Error is %s !", ticketConsumeRecord.TicketNumber, err)
			c.JSON(http.StatusOK, Http_error(models.ConsumeTicketFail, fmt.Sprintf("insert ticketConsumeRecord [%s] fail,Error is %s !", ticketConsumeRecord.TicketNumber, err.Error()), nil))
			return
		}
		logger.Debugf("insert ticketConsumeRecord [%s] success!", ticketConsumeRecord.TicketNumber)
		c.JSON(http.StatusOK, Http_success(models.ConsumeTicketSuccess, fmt.Sprintf("insert ticketConsumeRecord [%s] success!", ticketConsumeRecord.TicketNumber), nil))
		return
	} else {
		c.JSON(http.StatusOK, Http_error(models.ConsumeTicketFail, err.Error(), nil))
	}

}
