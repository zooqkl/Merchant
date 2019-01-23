/*
Copyright Yunphant Corp. All Rights Reserved.
*/

package msp

import (
	"fmt"
	"github.com/hyperledger/fabric/bccsp/factory"
	fabmsp "github.com/hyperledger/fabric/msp"
	mspcache "github.com/hyperledger/fabric/msp/cache"
	sdkconfig "github.com/hyperledger/fabric/sdk/config"
	fabbccsp "github.com/hyperledger/fabric/bccsp"
	"github.com/hyperledger/fabric/sdk/logging"
	"github.com/hyperledger/fabric/common/crypto"
	"os"
	"reflect"
	"unsafe"
)


type MspManager struct {
	msps map[string]fabmsp.MSP
}

/*
 作用：由msp目录初始化msp
*/
func NewMspManager(mspConfig sdkconfig.MSPConfig) (*MspManager,error) {
	_, err := os.Stat(mspConfig.UserPath)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("cannot init crypto, missing %s folder", mspConfig.UserPath)
	}

	theMsps := make(map[string]fabmsp.MSP)
	// err = mspmgmt.LoadLocalMsp(mspConfig.UserPath, bccspConfig, mspConfig.Id)
	// if err != nil {
	// 	return fmt.Errorf("error when setting up MSP from directory %s: err %s", mspConfig.UserPath, err)
	// }
	defaultBccspConf := newBccspConfig(mspConfig.BCCSP)
	defaultConf,err := fabmsp.GetLocalMspConfig(mspConfig.UserPath, defaultBccspConf, mspConfig.Id)
	if err != nil {
		return nil, fmt.Errorf("[InitCrypto] GetLocalMspConfig of default error: %s",err)
	}

	defaultMsp := newMSP(defaultBccspConf)
	err = defaultMsp.Setup(defaultConf)
	if err != nil {
		return nil, fmt.Errorf("[InitCrypto] Setup of default msp error: %s",err)
	}
	theMsps["default"] = defaultMsp

	for peerName,baseConf := range mspConfig.FakeMsp {
		bccspConfig := newBccspConfig(mspConfig.BCCSP)
		conf,err := fabmsp.GetLocalMspConfig(baseConf.Path, bccspConfig, baseConf.Id)
		if err != nil {
			return nil, fmt.Errorf("[InitCrypto] GetLocalMspConfig of [%s] error: %s", peerName, err)
		}

		msp := newMSP(bccspConfig)
		err = msp.Setup(conf)
		if err != nil {
			return nil, fmt.Errorf("[InitCrypto] Setup of [%s] msp error: %s", peerName, err)
		}
		theMsps[peerName] = msp
	}

	return &MspManager{msps:theMsps}, nil
}

func (mm *MspManager) GetDefaultSigningIdentity() (fabmsp.SigningIdentity, error) {
	defaultMsp,ok := mm.msps["default"]
	if !ok {
		return nil,fmt.Errorf("[GetDefaultSigningIdentity] defualt msp not inited!")
	}
	return defaultMsp.GetDefaultSigningIdentity()
}

func (mm *MspManager) GetSigningIdentity(key string) (fabmsp.SigningIdentity, error) {
	msp,ok := mm.msps[key]
	if !ok {
		sdklogging.GetLogger().Warnf("[GetSigningIdentity] Msp of %s not found in msp manager,use default msp instead.", key)
		msp,ok = mm.msps["default"]
		if !ok {
			return nil,fmt.Errorf("[GetDefaultSigningIdentity] defualt msp not inited!")
		}
	}
	return msp.GetDefaultSigningIdentity()
}

func (mm *MspManager) NewSigner(key string) crypto.LocalSigner {
	msp,ok := mm.msps[key]
	if !ok {
		sdklogging.GetLogger().Warnf("[NewSigner] Msp of %s not found in msp manager,use default msp instead.", key)
		msp = mm.msps["default"]
	}
	return &mspSigner{msp}
}

func newMSP(fOpts *factory.FactoryOpts) fabmsp.MSP {
	var lclMsp fabmsp.MSP

	mspType :=  fabmsp.ProviderTypeToString(fabmsp.FABRIC)

	// based on the MSP type, generate the new opts
	var newOpts fabmsp.NewOpts
	switch mspType {
	case fabmsp.ProviderTypeToString(fabmsp.FABRIC):
		newOpts = &fabmsp.BCCSPNewOpts{NewBaseOpts: fabmsp.NewBaseOpts{Version: fabmsp.MSPv1_0}}
	case fabmsp.ProviderTypeToString(fabmsp.IDEMIX):
		newOpts = &fabmsp.IdemixNewOpts{fabmsp.NewBaseOpts{Version: fabmsp.MSPv1_1}}
	default:
		panic("msp type " + mspType + " unknown")
	}

	var err error

	bccsp,err := factory.GetBCCSPFromOpts(fOpts)
	if err != nil {
		panic(fmt.Sprintf("Failed to GetBCCSPFromOpts,received error: %s",err))
	}

	mspInst, err := fabmsp.New(newOpts)
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize MSP, received error: %s", err))
	}
	// reset bccsp instance in mspInst or it will use default bccsp instance
	*(*fabbccsp.BCCSP)(unsafe.Pointer(reflect.ValueOf(mspInst).Elem().FieldByName("bccsp").UnsafeAddr())) = bccsp


	switch mspType {
	case fabmsp.ProviderTypeToString(fabmsp.FABRIC):
		lclMsp, err = mspcache.New(mspInst)
		if err != nil {
			panic(fmt.Sprintf("Failed to initialize cache MSP, received error: %s", err))
		}
	case fabmsp.ProviderTypeToString(fabmsp.IDEMIX):
		lclMsp = mspInst
	default:
		panic("msp type " + mspType + " unknown")
	}

	return lclMsp
}

func newBccspConfig(config sdkconfig.BCCSPConfig) *factory.FactoryOpts {
	return &factory.FactoryOpts{
		ProviderName: config.Provider,
		SwOpts: &factory.SwOpts{
			SecLevel:   config.Security,
			HashFamily: config.Hash,
			Ephemeral:  true,
			FileKeystore: &factory.FileKeystoreOpts{
				KeyStorePath: config.FileKeyStore.KeyStore,
			},
		},
	}
}