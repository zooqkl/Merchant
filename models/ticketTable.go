package models

import (
	"time"
	"github.com/jinzhu/gorm"
)

type TicketTable struct {
	gorm.Model
	TicketNumber       string
	TicketStatus       int
	TicketTime         time.Time
	TicketMerchantName string
}

func (TicketTable) TableName() string {
	return "ticket_table"
}
