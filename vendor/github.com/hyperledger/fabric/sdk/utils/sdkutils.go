package utils

import (
	"bytes"
	"github.com/hyperledger/fabric/common/util"
	"github.com/hyperledger/fabric/sdk/logging"
)

func PrettyPrintStruct(i interface{}) {
	params := util.Flatten(i)
	var buffer bytes.Buffer
	for i := range params {
		buffer.WriteString("\n\t")
		buffer.WriteString(params[i])
	}
	logger := sdklogging.GetLogger()
	logger.Infof("SDK config values:%s\n", buffer.String())
}
