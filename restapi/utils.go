package main

import (
	"regexp"
	"crypto/md5"
	"fmt"
	"github.com/gin-gonic/gin"
)

func CheckEmail(email string) (b bool) {
	reg := regexp.MustCompile("^[A-Za-z0-9!#$%&'+/=?^_`{|}~-]+(.[A-Za-z0-9!#$%&'+/=?^_`{|}~-]+)*@([A-Za-z0-9]+(?:-[A-Za-z0-9]+)?.)+[A-Za-z0-9]+(-[A-Za-z0-9]+)?$")
	return reg.MatchString(email)
}
func CheckPhone(mobileNum string) bool {
	reg := regexp.MustCompile("^(13[0-9]|14[57]|15[0-35-9]|18[07-9])\\d{8}$")
	return reg.MatchString(mobileNum)
}

func CheckPasswd(input string) bool {
	reg := regexp.MustCompile(`^([A-Z]|[a-z]|[0-9]|[-=[;,./~!@#$%^*()_+}{:?]){6,20}$`)
	return reg.MatchString(input)
}

func CheckName(input string) bool {
	reg := regexp.MustCompile(`^[a-z0-9A-Z\p{Han}]+(_[a-z0-9A-Z\p{Han}]+)*$`)
	return reg.MatchString(input)
}

func HashPasswd(pw string) string {
	result := fmt.Sprintf("%x", md5.Sum([]byte(pw)))
	return result
}

func Http_success(resultCode int, msg string, payload interface{}) gin.H {
	return gin.H{
		"result_code": resultCode,
		"result_msg":  msg,
		"payload":     payload,
	}
}

func Http_error(resultCode int, msg string, payload interface{}) gin.H {
	return gin.H{
		"result_code": resultCode,
		"result_msg":  msg,
		"payload":     payload,
	}
}
