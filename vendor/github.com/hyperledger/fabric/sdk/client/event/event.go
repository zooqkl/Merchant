/*
Copyright Yunphant Corp. All Rights Reserved.
*/

package event

import (
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/core/ledger/util"
	"github.com/hyperledger/fabric/protos/common"
	pb "github.com/hyperledger/fabric/protos/peer"
	"github.com/hyperledger/fabric/protos/utils"
	sdkClient "github.com/hyperledger/fabric/sdk/client"
	sdkPeer "github.com/hyperledger/fabric/sdk/client/peer"
	sdkConfig "github.com/hyperledger/fabric/sdk/config"
	fabmsp "github.com/hyperledger/fabric/msp"
	"github.com/hyperledger/fabric/sdk/logging"
	"github.com/hyperledger/fabric/sdk/utils/pathvar"
	"regexp"
	"sync"
	"time"
	"github.com/gorilla/websocket"
	"reflect"
)

// implement EventAdapter interface of fabric/events
// peer receive block -> EventsClient(consumer) -> EventHandler(adapter)
type EventHandler struct {
	connectStatusMtx     sync.Mutex
	retraceMtx           sync.Mutex
	txRegistrants        sync.Map
	chaincodeRegistrants sync.Map
	interestedEvents     []*pb.Interest
	eventClient          *EventsClient
	connected            bool
	peerConfig           sdkConfig.PeerConfig
	peerClient           *sdkPeer.PeerClient
	eventProcessor       *EventProcessor
	tpsCalculator        *tpsCounter
	signingIdentity      fabmsp.SigningIdentity

	wsConn               sync.Map
	uuidRegistrants      sync.Map
	workflowhistory      sync.Map
	handledworkflowhistory sync.Map
	workflowtxid 		 sync.Map
	workflowRegistrants  sync.Map
	uuidToCcName         sync.Map
	uuidToEventNames     sync.Map

}

var eventHandlers sync.Map
var globalEventHandler *EventHandler
var logger = sdklogging.GetLogger()

const (
	NOTIFY_TX_EVENT_DEFAULT_TIMEOUT = 10
	NOTIFY_CC_EVENT_DEFAULT_TIMEOUT = 10
)

func GetEventHandler(pc *sdkPeer.PeerClient, cf *sdkConfig.ConfigFactory, identity fabmsp.SigningIdentity) (*EventHandler, error) {
	if globalEventHandler != nil {
		return globalEventHandler, nil
	}
	eh, ok := eventHandlers.Load(pc.Name())
	if ok {
		globalEventHandler = eh.(*EventHandler)
		return eh.(*EventHandler), nil
	}

	neh := &EventHandler{
		peerConfig: cf.GetPeerConfig(pc.Name()),
		peerClient: pc,
		signingIdentity: identity,
	}

	neh.interestedEvents = append(neh.interestedEvents, &pb.Interest{EventType: pb.EventType_BLOCK})
	eventGrpcConfig := EventGrpcConfig{
		address:            sdkClient.GrpcAddressFromUrl(neh.peerConfig.EventAddress),
		serverNameOverride: neh.peerConfig.GrpcConfig.ServerNameOverride,
		tlsEnabled:         sdkClient.AttemptSecured(neh.peerConfig.EventAddress, true),
		certFile:           pathvar.Subst(neh.peerConfig.GrpcConfig.TlsCaCertPath),
	}

	channels, err := sdkPeer.GetChannels(pc, identity)
	if err != nil {
		return nil, fmt.Errorf("[Event] Get channels error: %s", err)
	}
	neh.eventProcessor = NewEventProcessor(channels, neh.seekBlock, neh.getHeight, neh.handleBlockEvent, cf.GetEventConfig().Retrace)
	neh.eventProcessor.StartService()

	eventClient, err := NewEventsClient(eventGrpcConfig, cf.GetEventTimeoutConfig().RegistrationResponse, neh, identity)
	if err != nil {
		return nil, fmt.Errorf("NewEventHandler create new event client error: %s", err)
	}

	err = eventClient.Start()
	if err != nil {
		neh.connected = false
		eventClient.Stop()
		return nil, fmt.Errorf("NewEventHandler start event client error: %s", err)
	}

	neh.eventClient = eventClient
	neh.connected = true

	globalEventHandler = neh
	eventHandlers.Store(pc.Name(), neh)
	return neh, nil
}

func (e *EventHandler) Reconnect() error {
	defer e.connectStatusMtx.Unlock()
	e.connectStatusMtx.Lock()
	if e.connected {
		return nil
	}
	err := e.eventClient.Start()
	if err != nil {
		e.connected = false
		e.eventClient.Stop()
	} else {
		e.connected = true
	}
	logger.Infof("Event client has reconnected to peer: %s", e.peerConfig.EventAddress)
	return err
}

func (e *EventHandler) RegisterTxEvent(txid string) <-chan TxEventInfo {
	eventData := make(chan TxEventInfo)
	e.txRegistrants.Store(txid, eventData)
	return eventData
}

// should call after got result or timeout
func (e *EventHandler) UnregisterTxEvent(txid string) {
	e.txRegistrants.Delete(txid)
}


// 判断obj是否在target中，target支持的类型arrary,slice,map
func Contain(obj interface{}, target interface{}) (bool, error) {
	targetValue := reflect.ValueOf(target)
	switch reflect.TypeOf(target).Kind() {
	case reflect.Slice, reflect.Array:
		for i := 0; i < targetValue.Len(); i++ {
			if targetValue.Index(i).Interface() == obj {
				return true, nil
			}
		}
	case reflect.Map:
		if targetValue.MapIndex(reflect.ValueOf(obj)).IsValid() {
			return true, nil
		}
	}

	return false, nil
}



func (e *EventHandler) RegitsterChaincodeEvent(ccid, uuid_connname string,eventnames []string) (<-chan ChaincodeEventInfo, chan int) {
	logger.Infof("registerchaincode event")
	eventData := make(chan ChaincodeEventInfo)
	errcode := make(chan int,1)

	var eventsInOneChaincode *sync.Map
	connected, ok := e.chaincodeRegistrants.Load(ccid)
	if ok && connected != nil {
		eventsInOneChaincode = connected.(*sync.Map)
	} else {
		eventsInOneChaincode = &sync.Map{}
	}
	//events of one chaincode, middle parameter is dappuid
	for _,eventname := range eventnames{
		eventsInOneChaincode.Store(eventname, eventData)
	}

	e.chaincodeRegistrants.Store(ccid, eventsInOneChaincode)
	e.uuidToCcName.Store(uuid_connname,ccid)
	e.uuidToEventNames.Store(uuid_connname,eventnames)

	var connsInOneChaincode *sync.Map
	connected, ok = e.uuidRegistrants.Load(ccid)
	if ok && connected != nil {
		connsInOneChaincode = connected.(*sync.Map)
	} else {
		connsInOneChaincode = &sync.Map{}
	}


	conn := e.GetWsConn(uuid_connname)
	if conn == nil{
		logger.Infof("cant get conn when registerchaincodeevent")
		errcode <- 40017
		return nil, errcode
	}

	oneconn,ok := conn.(*websocket.Conn)
	if !ok{
		logger.Infof("cant assert conn when registerchaincodeevent")
		errcode <- 40018
		return nil, errcode
	}
	//conns of one chaincode, middle parameter is dappuid
	logger.Infof("eventnames are:%v\n",eventnames)
	for _,eventname := range eventnames{
		logger.Infof("eventname is:%v\n",eventname)
		conns,ok := connsInOneChaincode.Load(eventname)
		if ok{
			tmpconns := conns.([]*websocket.Conn)
			if ok{
				contain,_ := Contain(oneconn,tmpconns)
				if ! contain{
					tmpconns = append(tmpconns, oneconn)
					connsInOneChaincode.Store(eventname,tmpconns)
				}

			}else{
				tmpconns = []*websocket.Conn{oneconn}
				connsInOneChaincode.Store(eventname,tmpconns)
			}
		}else{
			tmpconns := []*websocket.Conn{oneconn}
			connsInOneChaincode.Store(eventname,tmpconns)
		}

	}
	for _,eventname := range eventnames{
		conns,_ := connsInOneChaincode.Load(eventname)
		logger.Infof("eventname is:%v,conns is:%v\n",eventname,conns)
	}

	logger.Infof("connsInOneChaincode are:%v\n",connsInOneChaincode)

	e.uuidRegistrants.Store(ccid,connsInOneChaincode)
	errcode <- 40016
	return eventData, errcode
}

func (e *EventHandler) UnregitsterChaincodeEvent(ccid, eventname string) {
	eventsInOneChaincode, ok := e.chaincodeRegistrants.Load(ccid)
	if ok {
		eventsInOneChaincode.(*sync.Map).Delete(eventname)
	}
}

func (e *EventHandler) GetInterestedEvents() ([]*pb.Interest, error) {
	return e.interestedEvents, nil
}

func (e *EventHandler) Recv(msg *pb.Event) (bool, error) {
	go func() {
		switch msg.Event.(type) {
		case *pb.Event_Block:
			blockEvent := msg.Event.(*pb.Event_Block)
			channelId, err := getChannelId(blockEvent.Block)
			if err != nil {
				logger.Error(err)
				return
			}
			// logger.Infof("[Event - %s] Receive block event of number [%v]", channelId, blockEvent.Block.GetHeader().GetNumber())
			// if e.eventChecker.NeedProcess(channelId, blockEvent.Block.GetHeader().GetNumber()) {
			// 	e.handleBlockEvent(blockEvent.Block)
			// }
			e.eventProcessor.Process(channelId, blockEvent)
			return
		case *pb.Event_ChaincodeEvent:
			ccEvent := msg.Event.(*pb.Event_ChaincodeEvent)
			logger.Debugf("Recv chaincode event of txid: [%v]", ccEvent.ChaincodeEvent.GetTxId())
			if ccEvent != nil {
				e.handleChaincodeEvent("", ccEvent.ChaincodeEvent, false)
			}
			return
		default:
			return
		}
	}()

	return true, nil
}

func (e *EventHandler) Disconnected(err error) {
	e.setConnectStatus(false)
	if err == nil {
		logger.Info("Event client received io.EOF from stream,disconnect!")
		return
	}

	logger.Errorf("Event client disconnect from server with error: %s", err)
	// try to reconnect
	go func() {
		for i := 1; i <= 10; i++ {
			logger.Infof("Event client try to reconnect to peer [%s] for the %dst time", e.peerConfig.EventAddress, i)
			err := e.Reconnect()
			if err == nil {
				return
			}
			time.Sleep(time.Duration(i) * 5 * time.Second)
		}
		logger.Warnf("Event client has tried 10 times to reconnect to peer [%s],stop trying!", e.peerConfig.EventAddress)
	}()
}

func (e *EventHandler) setConnectStatus(status bool) {
	defer e.connectStatusMtx.Unlock()
	e.connectStatusMtx.Lock()
	e.connected = status
}

func (e *EventHandler) IsConnect() bool {
	defer e.connectStatusMtx.Unlock()
	e.connectStatusMtx.Lock()
	return e.connected
}

func (e *EventHandler) handleBlockEvent(block *common.Block) {
	txFilter := util.TxValidationFlags(block.Metadata.Metadata[common.BlockMetadataIndex_TRANSACTIONS_FILTER])
	if e.tpsCalculator != nil {
		e.tpsCalculator.accumulate(uint64(len(block.Data.Data)))
	}
	for i, tdata := range block.Data.Data {
		env, err := utils.GetEnvelopeFromBlock(tdata)
		if err != nil {
			logger.Errorf("[Event] Extracting Envelope from block error: %s\n", err)
			continue
		}

		// get the payload from the envelope
		payload, err := utils.GetPayload(env)
		if err != nil {
			logger.Errorf("[Event] Extracting Payload from envelope error: %s\n", err)
			return
		}
		channelHeaderBytes := payload.Header.ChannelHeader
		channelHeader := &common.ChannelHeader{}
		err = proto.Unmarshal(channelHeaderBytes, channelHeader)
		if err != nil {
			logger.Errorf("[Event - %s] Extracting ChannelHeader from payload error: %s\n", channelHeader.ChannelId, err)
			return
		}
		ch, ok := e.txRegistrants.Load(channelHeader.GetTxId())
		if ok {
			eventInfo := TxEventInfo{
				Txid:           channelHeader.GetTxId(),
				ValidationCode: txFilter.Flag(i).String(),
			}
			select {
			case ch.(chan TxEventInfo) <- eventInfo:
			case <-time.After(time.Duration(NOTIFY_TX_EVENT_DEFAULT_TIMEOUT) * time.Second):
				logger.Warnf("[Event - %s] Waiting for tx event reading until timeout,txid: %s", channelHeader.ChannelId, channelHeader.GetTxId())
			}
			close(ch.(chan TxEventInfo))
			e.txRegistrants.Delete(channelHeader.GetTxId())
		} else {
			logger.Debugf("[Event - %s]No registration found for TxID: %s", channelHeader.ChannelId, channelHeader.TxId)
		}

		// chaincode event
		if ccEvent, channelID, err := getChaincodeEvent(payload.Data, channelHeader); err != nil {
			logger.Warnf("getChainCodeEvent return error: %v\n", err)
		} else if ccEvent != nil {
			e.handleChaincodeEvent(channelID, ccEvent, true)
		}

	}
}

func (e *EventHandler) handleChaincodeEvent(channelID string, ccEvent *pb.ChaincodeEvent, patternMatch bool) {
	eventsInOneChaincode, ok := e.chaincodeRegistrants.Load(ccEvent.GetChaincodeId())
	if !ok {
		logger.Debugf("No registration found for chaincode: %s", ccEvent.GetChaincodeId())
		return
	}
	notifyCCEvent := func(key, value interface{}) bool {
		match := key.(string) == ccEvent.GetEventName()
		if !match && patternMatch {
			match, _ = regexp.MatchString(key.(string), ccEvent.GetEventName())
		}
		if match {
			//conns,ok := e.uuidRegistrants.Load(ccEvent.GetEventName())
			ccid := ccEvent.GetChaincodeId()
			connsInOneChaincode1, ok := e.uuidRegistrants.Load(ccid)
			conns,ok := connsInOneChaincode1.(*sync.Map).Load(ccEvent.GetEventName())
			logger.Infof("conns and eventname are:%v,---%v",conns,ccEvent.GetEventName())
			if !ok{
				logger.Infof("failed to load conn")
				return false
			}
			ch := value.(chan ChaincodeEventInfo)
			select {
			case ch <- ChaincodeEventInfo{
				ChaincodeID: ccEvent.GetChaincodeId(),
				TxID:        ccEvent.GetTxId(),
				EventName:   ccEvent.GetEventName(),
				Payload:     ccEvent.GetPayload(),
				ChannelID:   channelID,
			}:

			if key,ok := key.(string); ok{
				e.workflowhistory.Store(key,string(ccEvent.Payload[:]))
				e.workflowtxid.Store(ccEvent.GetTxId(),string(ccEvent.Payload[:]))
				conns,ok := conns.([]*websocket.Conn)
				var goodconns []*websocket.Conn
				for _,oneconn := range conns{
					if !ok{
						logger.Errorf("cant assert conns to websocket.conn")
						return false
					}
					err := oneconn.WriteMessage(websocket.TextMessage, ccEvent.GetPayload())
					if err !=nil{

						logger.Infof("write error is :%v\n",err)
						oneconn.CloseHandler()
						//a,_ :=e.uuidRegistrants.Load(ccEvent.GetChaincodeId())
						//a.(*sync.Map).Delete(ccEvent.GetEventName())
						oneconn.Close()
					}else{
						goodconns =append(goodconns, oneconn)
					}
				}
				connsInOneChaincode1.(*sync.Map).Store(ccEvent.GetEventName(),goodconns)
			}

			case <-time.After(time.Duration(NOTIFY_CC_EVENT_DEFAULT_TIMEOUT) * time.Second):
				logger.Warnf("Waiting for chaincode event reading until timeout,ccid: %s,eventName: %s", ccEvent.GetChaincodeId(), ccEvent.GetEventName())
				// need to close chan here?
				// close(value.(chan ChaincodeEventInfo))
			}
		}
		return true
	}
	eventsInOneChaincode.(*sync.Map).Range(notifyCCEvent)
}

func (e *EventHandler) seekBlock(channel string, num uint64) {
	e.retraceMtx.Lock()
	defer e.retraceMtx.Unlock()
	logger.Debugf("[Event - %s] Seeking block [%d]", channel, num)
	block, err := sdkPeer.GetBlockByNumber(e.peerClient, channel, num, e.signingIdentity)
	if err != nil {
		logger.Errorf("[Event - %s] Seek block [%v] error: %s", channel, num, err)
		return
	}
	event := &pb.Event{Event: &pb.Event_Block{Block: block}}
	e.Recv(event)
}

func (e *EventHandler) getHeight(channel string) (uint64, error) {
	e.retraceMtx.Lock()
	defer e.retraceMtx.Unlock()
	chainInfo, err := sdkPeer.GetChainInfo(e.peerClient, channel, e.signingIdentity)
	if err != nil {
		return 0, fmt.Errorf("[Event - %s] Get height error: %s", channel, err)
	}
	return chainInfo.GetHeight(), nil
}

func (e *EventHandler) GetTpsOutput() (<-chan uint64, error) {
	if e.tpsCalculator != nil {
		return nil, fmt.Errorf("Tps output channel can only be read by one routine!")
	}
	e.tpsCalculator = calculateTps()
	return e.tpsCalculator.output, nil
}

// getChainCodeEvents parses block events for chaincode events associated with individual transactions
func getChaincodeEvent(payloadData []byte, channelHeader *common.ChannelHeader) (event *pb.ChaincodeEvent, channelID string, err error) {
	// Chaincode events apply to endorser transaction only
	if common.HeaderType(channelHeader.Type) == common.HeaderType_ENDORSER_TRANSACTION {
		tx, err := utils.GetTransaction(payloadData)
		if err != nil {
			return nil, "", fmt.Errorf("unmarshal transaction payload: %s", err)
		}
		chaincodeActionPayload, err := utils.GetChaincodeActionPayload(tx.Actions[0].Payload)
		if err != nil {
			return nil, "", fmt.Errorf("chaincode action payload retrieval failed: %s", err)
		}
		propRespPayload, err := utils.GetProposalResponsePayload(chaincodeActionPayload.Action.ProposalResponsePayload)
		if err != nil {
			return nil, "", fmt.Errorf("proposal response payload retrieval failed: %s", err)
		}
		caPayload, err := utils.GetChaincodeAction(propRespPayload.Extension)
		if err != nil {
			return nil, "", fmt.Errorf("chaincode action retrieval failed: %s", err)
		}
		ccEvent, err := utils.GetChaincodeEvents(caPayload.Events)

		if ccEvent != nil {
			return ccEvent, channelHeader.ChannelId, nil
		}

	}
	return nil, "", nil
}

func getChannelId(block *common.Block) (string, error) {
	if len(block.Data.Data) <= 0 {
		return "", fmt.Errorf("[Event] GetchannelHeader error: length of block data is less than 1")
	}
	if env, err := utils.GetEnvelopeFromBlock(block.Data.Data[0]); err != nil {
		return "", fmt.Errorf("[Event] GetEnvelopeFromBlock error: %s", err)
	} else {
		payload, err := utils.GetPayload(env)
		if err != nil {
			return "", fmt.Errorf("[Event] Extract payload from envelope failed: %s", err)
		}
		channelHeaderBytes := payload.Header.ChannelHeader
		channelHeader := &common.ChannelHeader{}
		err = proto.Unmarshal(channelHeaderBytes, channelHeader)
		if err != nil {
			return "", fmt.Errorf("[Event] Unmarshal channel header failed: %s", err)
		}
		return channelHeader.GetChannelId(), nil
	}
}






func (e *EventHandler)SetWsConn(uuid string,conn interface{}) error {

	e.wsConn.Store(uuid,conn)
	conn1,ok := conn.(*websocket.Conn)
	if !ok{
		logger.Infof("setwsconn fail")
		return fmt.Errorf("setwsconn fail")
	}
	conn1.WriteMessage(1,[]byte("setwsconn success"))
	logger.Infof("setwsconn good, uuid is %v\n",uuid)
	return nil
}



func (e *EventHandler)GetChaincodeRegistrants(ccid string) interface{} {
	res,ok := e.chaincodeRegistrants.Load(ccid)
	if !ok{
		logger.Infof("cant get uuidregistrants by %s\n",ccid)
		return nil
	}
	return res
}

func (e *EventHandler)GetUuidRegistrants(eventname string) (interface{}) {
	res,ok := e.uuidRegistrants.Load(eventname)
	if !ok{
		logger.Infof("cant get uuidregistrants by %s\n",eventname)
		return nil
	}
	return res
}



func (e *EventHandler)GetWsConn(uuid string) interface{} {
	conn,ok := e.wsConn.Load(uuid)
	if !ok{
		logger.Infof("cant get wsconn by %s\n",uuid)
		return nil
	}
	return conn
}


func (e *EventHandler)GetCcName(uuid string) interface{} {
	ccname,ok := e.uuidToCcName.Load(uuid)
	logger.Infof("uuid is:%s,ccname is:%s,uuidtoccname is:%v\n",uuid,ccname,e.uuidToCcName)

	if !ok{
		logger.Infof("cant get wsconn by %s\n",uuid)
		return nil
	}
	return ccname
}

func (e *EventHandler)GetEventNames(uuid string) interface{} {
	eventnames,ok := e.uuidToEventNames.Load(uuid)
	logger.Infof("uuid is:%s,eventnames is:%s,uuidtoccname is:%v\n",uuid,eventnames,e.uuidToCcName)

	if !ok{
		logger.Infof("cant get wsconn by %s\n",uuid)
		return nil
	}
	return eventnames
}