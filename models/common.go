package models

const (
	// Ticket status
	VAILD    int = 0
	USED     int = 1
	INVAILD  int = 2
	DISABLED int = 3

	// user status
	AVAILABLE   int = 1
	UNAVAILABLE int = 0

	// 用户注册返回状态码
	FormatError       int = 4000
	UserIsExits       int = 4001
	UserIsNotExits    int = 4002
	CreateUserSuccess int = 2000
	CreateUserFail    int = 2001

	//用户更改密码返回状态码
	ChangePasswdSuccess int = 2000
	ChangePasswdFail    int = 2001

	// 用户登录状态码
	UserLoginSuccess int = 2000
	UserLoginFail    int = 2001

	ServerInternalErr int = 5000

	//票券查询返回状态码
	QueryTicketSuccess int = 2000
	QueryTicketFail    int = 2001

	//票券消费返回状态码
	ConsumeTicketSuccess int = 2000
	ConsumeTicketFail    int = 2001
)

// Error message
var (
	UserNameOrPasswdError string = "username or password error！"
	UsernameFormatError   string = "name format Error,Expecting Email or phone !"
	PasswdFormatError     string = "Password is  wrong format！"
	PasswdError           string = "Password verify fail!please input correct password!"
	ServerInternalError   string = "Server internal Error,please later again!"
	UserIsExitsError      string = "Username is already occupied，Please change the name！"
)
