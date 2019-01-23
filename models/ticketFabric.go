package models
import "time"

type Ticket struct {
	TicketNumber       string       `json:"ticket_number,omitempty"`
	TicketStatus       TicketStatus `json:"ticket_status,omitempty"`
	TicketTime         time.Time    `json:"ticket_time,omitempty"`
	TicketMerchantName string       `json:"ticket_merchant_name"`
	CreateTime         time.Time    `json:"create_time,omitempty"`
	LastUpdateTime     time.Time    `json:"last_update_time,omitempty"`
}

type TicketStatus int
const (
	TicketStatus__Vaild   TicketStatus = iota + 1
	TicketStatus__Invaild
	TicketStatus__Used
	TicketStatus__Disable
)

var rStatus = []string{
	"有效",
	"已失效",
	"已使用",
	"不可用",
}

func (t TicketStatus) String() string {
	return rStatus[t-1]
}