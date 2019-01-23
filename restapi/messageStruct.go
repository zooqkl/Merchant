package main

type Respond struct {
	ResultCode string `json:"resultCode"`
	ResultMsg  string `json:"resultMsg"`
}

type LoginRequest struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

type ChangePasswdRequest struct {
	Name            string `json:"name"`
	OldPassword     string `json:"old_password"`
	NewPassword     string `json:"new_password"`
	ConfirmPassword string `json:"confirm_password"`
}

type RegisterRequest struct {
	Name            string `json:"name"`
	Password        string `json:"password"`
	MerchantName    string `json:"merchant_name"`
	MerchantAddress string `json:"merchant_address"`
	MerchantPhone   string `json:"merchant_phone"`
}
type TicketQueryRequest struct {
	TicketNumber string `json:"ticket_number"`
}
type TicketConsumeRequest struct {
	TicketNumber string `json:"ticket_number"`
}
