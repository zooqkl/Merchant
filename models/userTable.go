package models

import (
	"github.com/jinzhu/gorm"
)

type UserTable struct {
	gorm.Model
	//MerchantId      int64  `gorm:"index;AUTO_INCREMENT"`
	Username        string //`sql:"unique_index"` //使用邮箱或手机号
	Password        string
	MerchantName    string
	MerchantAddress string
	MerchantPhone   string
	LastLoginTime   string
	IsAvailable     int
}

func (UserTable) TableName() string {
	return "user_table"
}
