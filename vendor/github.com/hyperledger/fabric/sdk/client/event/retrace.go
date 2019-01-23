/*
Copyright Yunphant Corp. All Rights Reserved.
*/

package event

import (
	"encoding/json"
	"fmt"
	"github.com/hyperledger/fabric/protos/common"
	pb "github.com/hyperledger/fabric/protos/peer"
	"github.com/hyperledger/fabric/sdk/config"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const RETRACE_FILE_NAME = "./retrace.json"
const BLOCK_DISPATCH_CACHE_SIZE = 30

var (
	// 定时同步最新区块高度的周期
	SYNC_LATEST_INTERVAL          time.Duration
	PERSISTENCE_INTERVAL          = 5 * time.Second
	MAX_COUNT_TO_RETRACE_AT_START int

	globalEventProcessor *EventProcessor
)

type waitingPool struct {
	sync.Mutex
	m map[string]*pb.Event_Block
}

func (wp *waitingPool) put(channel string, num uint64, event *pb.Event_Block) {
	wp.Lock()
	defer wp.Unlock()
	if wp.m == nil {
		wp.m = make(map[string]*pb.Event_Block)
	}
	key := fmt.Sprintf("%s-%d", channel, num)
	wp.m[key] = event
}

func (wp *waitingPool) tryToGetAndDelete(channel string, num uint64) (*pb.Event_Block, bool) {
	wp.Lock()
	defer wp.Unlock()
	key := fmt.Sprintf("%s-%d", channel, num)
	event, ok := wp.m[key]
	if ok {
		delete(wp.m, key)
	}
	return event, ok
}

func (wp *waitingPool) isExist(channel string, num uint64) bool {
	wp.Lock()
	defer wp.Unlock()
	key := fmt.Sprintf("%s-%d", channel, num)
	_, ok := wp.m[key]
	return ok
}

func (wp *waitingPool) keys() map[string][]uint64 {
	wp.Lock()
	defer wp.Unlock()
	keys := make(map[string][]uint64)
	for k, _ := range wp.m {
		splits := strings.Split(k, "-")
		channel := splits[0]
		num, err := strconv.ParseUint(splits[1], 10, 64)
		if err != nil {
			logger.Errorf("[Event] WaitingPool parse uint64 error: %s", err)
		}
		keys[channel] = append(keys[channel], num)
	}
	return keys
}

type seekingBlockManager struct {
	sync.Mutex
	seeking map[string]string
}

func (sm *seekingBlockManager) seekBlock(channel string, num uint64, doSeek func(string, uint64)) {
	sm.Lock()
	defer sm.Unlock()
	key := fmt.Sprintf("%s-%d", channel, num)
	if _, ok := sm.seeking[key]; ok {
		return
	}
	sm.seeking[key] = key
	go func() {
		doSeek(channel, num)
		sm.Lock()
		delete(sm.seeking, key)
		sm.Unlock()
	}()
	go func() {
		time.Sleep(5 * time.Second)
		sm.Lock()
		delete(sm.seeking, key)
		sm.Unlock()
	}()
}

func initParams(cfg config.RetraceConfig) {
	// 定时同步最新区块高度的周期
	SYNC_LATEST_INTERVAL = cfg.SyncInterval
	// PERSISTENCE_INTERVAL = cfg.PersistenceInterval

	MAX_COUNT_TO_RETRACE_AT_START = cfg.MaxCountToRetraceAtStart
	logger.Debugf("[Event] Retrace config is SYNC_LATEST_INTERVAL:%v; PERSISTENCE_INTERVAL:%v; MAX_COUNT_TO_RETRACE_AT_START:%v", SYNC_LATEST_INTERVAL, PERSISTENCE_INTERVAL, MAX_COUNT_TO_RETRACE_AT_START)
}

func NewEventProcessor(channels []string, seekBlock func(string, uint64), getHeight func(string) (uint64, error), handleBlock func(*common.Block), cfg config.RetraceConfig) *EventProcessor {
	if globalEventProcessor != nil {
		logger.Warn("[Event] NewEventChecker function should not be called more than once!")
		return globalEventProcessor
	}

	initParams(cfg)

	globalEventProcessor = &EventProcessor{
		cache:               &waitingPool{m: make(map[string]*pb.Event_Block)},
		seekBlock:           seekBlock,
		getHeight:           getHeight,
		handleEventCallback: handleBlock,
	}
	globalEventProcessor.initialize(channels)

	return globalEventProcessor
}

type EventProcessor struct {
	sync.Mutex
	channelHandled      map[string]uint64
	cache               *waitingPool
	dispatcher          map[string]chan *common.Block
	seekBlock           func(string, uint64)
	getHeight           func(string) (uint64, error)
	handleEventCallback func(*common.Block)
	seekBlockManager    *seekingBlockManager
}

func (ech *EventProcessor) initialize(channels []string) {
	logger.Debug("[Event] Init event checker from history...")
	if ech.channelHandled == nil {
		ech.channelHandled = make(map[string]uint64)
	}
	if ech.dispatcher == nil {
		ech.dispatcher = make(map[string]chan *common.Block)
	}
	if ech.seekBlockManager == nil {
		ech.seekBlockManager = &seekingBlockManager{
			seeking: make(map[string]string),
		}
	}

	_, err := os.Stat(RETRACE_FILE_NAME)
	if os.IsNotExist(err) {
		logger.Debugf(`[Event] Can't find serialization file "%s",init event checker for the first time!`, RETRACE_FILE_NAME)
		ioutil.WriteFile(RETRACE_FILE_NAME, []byte("{}"), 0644)
	} else {
		serialization, err := ioutil.ReadFile(RETRACE_FILE_NAME)
		if err != nil {
			panic(fmt.Sprintf(`[Event] Initialize retrace history from "%s" error: %s`, RETRACE_FILE_NAME, err))
		}
		err = json.Unmarshal(serialization, &ech.channelHandled)
		if err != nil {
			panic(fmt.Sprintf(`[Event] Unmarshal history handled block error: %s`, err))
		}
	}

	for _, channel := range channels {
		if _, ok := ech.channelHandled[channel]; !ok {
			ech.channelHandled[channel] = 0
		}
	}

	for channel, latest := range ech.channelHandled {
		height, err := ech.getHeight(channel)
		if err != nil {
			panic(fmt.Sprintf(`[Event - %s] Initialize retrace getting height error: %s`, channel, err))
		}

		var threshold uint64
		if MAX_COUNT_TO_RETRACE_AT_START <= 0 {
			threshold = height - 1
		} else if height > uint64(MAX_COUNT_TO_RETRACE_AT_START+1) {
			threshold = height - uint64(MAX_COUNT_TO_RETRACE_AT_START) - 1
		} else {
			threshold = 0
		}

		if latest < threshold && threshold > 0 {
			ech.channelHandled[channel] = threshold
		}

		if height > 0 && height-1 != ech.channelHandled[channel] {
			ech.seekBlockManager.seekBlock(channel, height-1, ech.seekBlock)
		}
	}
	logger.Infof("[Event] Initialized result of channelHandled: %v", ech.channelHandled)
}

func (ech *EventProcessor) startPersistenceRoutine() {
	defer logger.Warnf("[Event] Persistence routine exit!")
	for {
		time.Sleep(PERSISTENCE_INTERVAL)
		logger.Debug("[Event] Writing event handled info to disk...")
		ech.persistData()
	}
}

func (ech *EventProcessor) persistData() {
	ech.Lock()
	defer ech.Unlock()
	snapShot, err := json.Marshal(ech.channelHandled)
	if len(ech.channelHandled) == 0 || len(snapShot) == 0 {
		snapShot = []byte("{}")
	}
	if err != nil {
		logger.Errorf("[Event] Serialization error when marshaling channelHandled: %s", err)
		return
	}
	logger.Debugf("[Event] Start persist data to disk,content: %s", string(snapShot))
	f, err := ioutil.TempFile(filepath.Dir(RETRACE_FILE_NAME), fmt.Sprintf(".%s.tmp", filepath.Base(RETRACE_FILE_NAME)))
	if err != nil {
		logger.Errorf("[Event] Serialization creating temp file error: %s", err)
		return
	}
	if _, err := f.Write(snapShot); err != nil {
		f.Close()
		os.Remove(f.Name())
		logger.Errorf("[Event] Serialization writing to temp file error: %s", err)
		return
	}
	f.Close()
	err = os.Rename(f.Name(), RETRACE_FILE_NAME)
	if err != nil {
		logger.Errorf("[Event] Rename temp file to file error: %s", err)
		return
	}
	logger.Debug("[Event] Data persisted successful!")
}

func (ech *EventProcessor) startSyncLatestRoutine() {
	defer logger.Warnf("[Event] SyncLatest routine exit!")
	for {
		time.Sleep(SYNC_LATEST_INTERVAL)
		ech.Lock()
		channels := make([]string, 0)
		for channel, _ := range ech.channelHandled {
			channels = append(channels, channel)
		}
		ech.Unlock()

		for _, channel := range channels {
			height, err := ech.getHeight(channel)
			if err != nil {
				logger.Errorf("[Event-%s] Get height error: %s", channel, err)
				continue
			}
			logger.Debugf("[Event-%s] syncLatest: latest block hight is: %d,sdk latest is: %d", channel, height, ech.channelHandled[channel])

			ech.Lock()
			if (height-1) >= 0 && ech.channelHandled[channel] < (height-1) {
				logger.Debugf("[Event-%s] Get latest block: %d", channel, height-1)
				ech.seekBlockManager.seekBlock(channel, height-1, ech.seekBlock)
			}
			ech.Unlock()
		}
	}
}

func (ech *EventProcessor) StartService() {
	// go ech.startPersistenceRoutine()
	go ech.startSyncLatestRoutine()
}

func (ech *EventProcessor) Process(channel string, newEvent *pb.Event_Block) {
	newNumber := newEvent.Block.GetHeader().GetNumber()
	ech.Lock()
	latest, ok := ech.channelHandled[channel]
	ech.Unlock()

	if !ok {
		ech.Lock()
		ech.channelHandled[channel] = newNumber
		ech.Unlock()
		logger.Infof("[Event - %s] Received new event of [%d],latest handled is [%d]", channel, newNumber, latest)
		ech.dispatchBlockEvent(channel, newEvent.Block)
		return
	}
	logger.Infof("[Event - %s] Received new event of [%d],latest handled is [%d]", channel, newNumber, latest)
	if (latest + 1) == newNumber {
		ech.dispatchBlockEvent(channel, newEvent.Block)
		ech.Lock()
		ech.channelHandled[channel] = newNumber
		ech.Unlock()
		for i := (newNumber + 1); ; i++ {
			if event, ok := ech.cache.tryToGetAndDelete(channel, i); ok {
				ech.dispatchBlockEvent(channel, event.Block)
				ech.Lock()
				ech.channelHandled[channel] = i
				ech.Unlock()
			} else {
				break
			}
		}
	} else if (latest + 1) < newNumber {
		ech.cache.put(channel, newNumber, newEvent)
		for i := (latest + 1); i < newNumber; i++ {
			if !ech.cache.isExist(channel, i) {
				ech.seekBlockManager.seekBlock(channel, i, ech.seekBlock)
			}
		}
	} else {
		logger.Warnf("[Event-%s] New block [%d] received but already handled.", channel, newNumber)
	}
}

func (ech *EventProcessor) GetWaitingPool(channel string) []uint64 {
	m := ech.cache.keys()
	if _, ok := m[channel]; !ok {
		return []uint64{}
	}
	return m[channel]
}

func (ech *EventProcessor) dispatchBlockEvent(channel string, block *common.Block) {
	var ch chan *common.Block
	var ok bool
	ech.Lock()
	ch, ok = ech.dispatcher[channel]
	ech.Unlock()
	if !ok {
		ech.Lock()
		ech.dispatcher[channel] = make(chan *common.Block, BLOCK_DISPATCH_CACHE_SIZE)
		ch = ech.dispatcher[channel]
		ech.Unlock()
		go func() {
			defer logger.Errorf("[Event] Dispatcher routine of [%s] has exited unexpectly!", channel)
			logger.Infof("[Event] Dispatcher routine of [%s] started!", channel)
			for {
				block := <-ch
				logger.Debugf("[Event - %s] Processing block event [%d]", channel, block.GetHeader().GetNumber())
				ech.handleEventCallback(block)
				ech.persistData()
			}
		}()
	}
	ch <- block
}

// 下面的已废弃
// /*
// 变量:
//   - latest: 最近收到的区块号，上一个处理的区块号
//   - new: 刚到的待处理的区块号
//   - missingMap: 存储丢失的区块号
// 主协程:
//   1. new==latest, 忽略;
//   2. new>latest, 将new与latest之间的区块都丢到 missingMap 中并更新 latest;
//   3. new<latest, 查找 missingMap,如果命中并未被处理则当做收到新区快处理,如果未命中打印一条warn
// 回溯协程:
//   1. 每隔一段时间回溯 missingMap 中的区块
//   2. 每隔一段时间将内存数据写入硬盘
// 内存回收协程：
//   1. 每个丢失的区块都会存到missingMap中，且自身维护一个生命周期，生命周期到了之后会向管理协程发送死亡信号
//   2. 生命周期管理协程监听死亡信号，并将其从missMap中删除
// 区块同步协程：
//   1. 为避免网络连接断开，客户端长期不发交易，无法获取到最新区块的信息，需定时请求最新区块更新latest
// 初始化:
//   1. 读取文件中latest的值，并查询最新区块高度height
//   2. 读取配置中maxCountToRetraceAtStart的变量并更新latest
//   3. 如果height-1大于latest,读取最新区块
// */
//
// const RETRACE_FILE_NAME = "./retrace.json"
//
// var (
// 	// 区块回溯周期,每隔固定时间会回溯 missingMap 中的区块
// 	RETRACE_INTERVAL time.Duration
// 	// 丢失区块生命周期，每个丢失的区块会在 missingMap 中生存一段时间，以便回溯协程来查询
// 	MISSING_NUM_LIFECYCLE time.Duration
// 	// 定时同步最新区块高度的周期
// 	SYNC_LATEST_INTERVAL time.Duration
// 	PERSISTENCE_INTERVAL time.Duration
//
// 	maxCountToRetraceAtStart int
// )
//
// type missingStatus int
//
// const (
// 	inited missingStatus = iota
// 	seeking
// 	retraced
// )
//
// type missingBlock struct {
// 	number uint64
// 	status missingStatus
// 	heaven chan<- uint64
// }
//
// func newMissingBlock(num uint64, lifeManager chan uint64) *missingBlock {
// 	m := &missingBlock{
// 		number: num,
// 		status: inited,
// 		heaven: lifeManager,
// 	}
// 	m.setStatus(seeking)
// 	return m
// }
//
// func (m *missingBlock) setStatus(status missingStatus) {
// 	if m.status == inited {
// 		go func() {
// 			time.Sleep(MISSING_NUM_LIFECYCLE)
// 			m.heaven <- m.number
// 		}()
// 	}
// 	m.status = status
// }
//
// func (m *missingBlock) getStatus() missingStatus {
// 	return m.status
// }
//
// type EventChecker struct {
// 	sync.Mutex
// 	missingStore map[string]*channelMissingStore
// 	seekBlock    func(string, uint64)
// 	getHeight    func(string) (uint64, error)
// }
//
// func initParams(cfg config.RetraceConfig) {
// 	RETRACE_INTERVAL = cfg.RetraceInterval
// 	// 丢失区块生命周期，每个丢失的区块会在 missingMap 中生存一段时间，以便回溯协程来查询
// 	MISSING_NUM_LIFECYCLE = cfg.MissingBlockLife
//
// 	// 定时同步最新区块高度的周期
// 	SYNC_LATEST_INTERVAL = cfg.SyncInterval
// 	PERSISTENCE_INTERVAL = cfg.PersistenceInterval
//
// 	maxCountToRetraceAtStart = cfg.MaxCountToRetraceAtStart
// 	logger.Debugf("[Event] Retrace config is RETRACE_INTERVAL:%v; MISSING_NUM_LIFECYCLE:%v; SYNC_LATEST_INTERVAL:%v; PERSISTENCE_INTERVAL:%v; maxCountToRetraceAtStart:%v", RETRACE_INTERVAL, MISSING_NUM_LIFECYCLE, SYNC_LATEST_INTERVAL, PERSISTENCE_INTERVAL, maxCountToRetraceAtStart)
// }
//
// func StartRetrace(channels []string, seekBlock func(string, uint64), getHeight func(string) (uint64, error), cfg config.RetraceConfig) *EventChecker {
// 	initParams(cfg)
// 	eventChecker := &EventChecker{
// 		missingStore: make(map[string]*channelMissingStore),
// 		seekBlock:    seekBlock,
// 		getHeight:    getHeight,
// 	}
// 	eventChecker.initFromdisk()
// 	for _, channel := range channels {
// 		if _, ok := eventChecker.missingStore[channel]; !ok {
// 			height, err := getHeight(channel)
// 			if err != nil {
// 				logger.Errorf("[Event-%s] Get height error: %s", channel, err)
// 				continue
// 			}
// 			if height < 1 {
// 				continue
// 			}
// 			eventChecker.missingStore[channel] = NewChannelMissingStore(channel, height-1, seekBlock, getHeight)
// 		}
// 	}
// 	go eventChecker.persistData()
// 	return eventChecker
// }
//
// func (e *EventChecker) NeedProcess(channel string, new uint64) bool {
// 	e.Lock()
// 	defer e.Unlock()
// 	if cms, ok := e.missingStore[channel]; ok {
// 		return cms.ProcessAndCheck(new)
// 	}
// 	e.missingStore[channel] = NewChannelMissingStore(channel, new, e.seekBlock, e.getHeight)
// 	return true
// }
//
// func (e *EventChecker) PrintMissingMap() {
// 	e.Lock()
// 	defer e.Unlock()
// 	status := []string{"inited", "seeking", "retraced"}
// 	missMap := make(map[string][]string)
// 	for ch, ms := range e.missingStore {
// 		missings := make([]string, 0)
// 		for num, s := range ms.missingMap {
// 			missings = append(missings, fmt.Sprintf("%d:%s", num, status[int(s.status)]))
// 		}
// 		missMap[ch] = missings
// 	}
//
// 	logger.Debugf("Missing map: %v", missMap)
// }
//
// func (e *EventChecker) initFromdisk() {
// 	logger.Debug("[Event] Init event checker from history...")
// 	snapShot := make(map[string]*Serialization)
// 	_, err := os.Stat(RETRACE_FILE_NAME)
// 	if os.IsNotExist(err) {
// 		logger.Debugf(`[Event] Can't find serialization file "%s",init event checker for the first time!`, RETRACE_FILE_NAME)
// 	} else {
// 		serialization, err := ioutil.ReadFile(RETRACE_FILE_NAME)
// 		if err != nil {
// 			panic(fmt.Sprintf(`[Event] Initialize retrace history from "%s" error: %s`, RETRACE_FILE_NAME, err))
// 		}
// 		err = json.Unmarshal(serialization, &snapShot)
// 		if err != nil {
// 			panic(fmt.Sprintf(`[Event] Unmarshal history missing block error: %s`, err))
// 		}
// 	}
//
// 	for channel, serialization := range snapShot {
// 		height, err := e.getHeight(channel)
// 		if err != nil {
// 			panic(fmt.Sprintf(`[Event-%s] Initialize retrace getting height error: %s`, channel, err))
// 		}
//
// 		var threshold uint64
// 		if maxCountToRetraceAtStart <= 0 {
// 			threshold = height
// 		} else if height > uint64(maxCountToRetraceAtStart+1) {
// 			threshold = height - uint64(maxCountToRetraceAtStart) - 1
// 		} else {
// 			threshold = 0
// 		}
//
// 		if serialization.Latest < threshold && threshold > 0 {
// 			serialization.Latest = threshold
// 		}
//
// 		for k, v := range serialization.MissingBlock {
// 			if k < threshold || v != int(seeking) {
// 				delete(serialization.MissingBlock, k)
// 			}
// 		}
//
// 		e.missingStore[channel] = NewChannelMissingStore(channel, serialization.Latest, e.seekBlock, e.getHeight)
// 		e.missingStore[channel].SetMissingMap(serialization.MissingBlock)
// 		logger.Debugf("[Event-%s] Missing store inited: latest:%v, missingMap:%v", channel, e.missingStore[channel].latest, e.missingStore[channel].missingMap)
//
// 		if height > 0 {
// 			go e.seekBlock(channel, height-1)
// 		}
// 	}
// }
//
// func (e *EventChecker) persistData() {
// 	defer logger.Warnf("[Event] Persistence routine exit!")
// 	for {
// 		time.Sleep(PERSISTENCE_INTERVAL)
// 		logger.Debug("[Event] Writing event data to disk...")
// 		snapShot := make(map[string]*Serialization)
// 		for ch, m := range e.missingStore {
// 			snapShot[ch] = m.GetSnapshot()
// 		}
// 		serialization, err := json.Marshal(snapShot)
// 		if err != nil {
// 			logger.Errorf("[Event] Serialization error: %s", err)
// 		}
// 		err = ioutil.WriteFile(RETRACE_FILE_NAME, serialization, 0644)
// 		if err != nil {
// 			logger.Errorf("[Event] Write serialization to file error: %s", err)
// 		}
// 	}
// }
//
// type Serialization struct {
// 	Latest       uint64         `json:"latest"`
// 	MissingBlock map[uint64]int `json:"missingBlock"`
// }
//
// type channelMissingStore struct {
// 	sync.Mutex
// 	channel     string
// 	latest      uint64
// 	missingMap  map[uint64]*missingBlock
// 	lifeManager chan uint64
// 	seekBlock   func(channel string, num uint64)
// 	getHeight   func(channel string) (uint64, error)
// }
//
// func NewChannelMissingStore(channel string, current uint64, seekBlock func(string, uint64), getHeight func(string) (uint64, error)) *channelMissingStore {
// 	cms := &channelMissingStore{
// 		channel:     channel,
// 		latest:      current,
// 		missingMap:  make(map[uint64]*missingBlock),
// 		lifeManager: make(chan uint64),
// 		seekBlock:   seekBlock,
// 		getHeight:   getHeight,
// 	}
// 	go cms.retrace()
// 	go cms.manageLife()
// 	go cms.syncLatest()
// 	return cms
// }
//
// func (cms *channelMissingStore) ProcessAndCheck(new uint64) bool {
// 	cms.Lock()
// 	defer cms.Unlock()
// 	if new > cms.latest {
// 		for i := cms.latest + 1; i < new; i++ {
// 			cms.missingMap[i] = newMissingBlock(i, cms.lifeManager)
// 		}
// 		cms.latest = new
// 		return true
// 	} else if new < cms.latest {
// 		if mB, ok := cms.missingMap[new]; ok {
// 			if mB.getStatus() == seeking {
// 				mB.setStatus(retraced)
// 				return true
// 			}
// 			return false
// 		} else {
// 			logger.Warnf("[Event-%s] Missed block [%d] has been retraced but not match any key in missingMap,might be origin event.", cms.channel, new)
// 			return false
// 		}
// 	} else {
// 		logger.Warnf("[Event-%s] New block [%d] received with a same number of latest.", cms.channel, new)
// 		return false
// 	}
// }
//
// func (cms *channelMissingStore) manageLife() {
// 	defer logger.Warnf("[Event-%s] Life manager routine exit!", cms.channel)
// 	for {
// 		dead := <-cms.lifeManager
// 		logger.Debugf("[Event-%s] [%d] is dead,remove it from missingMap.", cms.channel, dead)
// 		cms.Lock()
// 		delete(cms.missingMap, dead)
// 		cms.Unlock()
// 	}
// }
//
// func (cms *channelMissingStore) retrace() {
// 	defer logger.Warnf("[Event-%s] Retrace routine exit!", cms.channel)
// 	for {
// 		time.Sleep(RETRACE_INTERVAL)
// 		cms.Lock()
// 		for num, mB := range cms.missingMap {
// 			if mB.status != retraced {
// 				logger.Debugf(`[Event-%s] Start Retrace block "%v"`, cms.channel, num)
// 				go cms.seekBlock(cms.channel, num)
// 			}
// 		}
// 		cms.Unlock()
// 	}
// }
//
// func (cms *channelMissingStore) syncLatest() {
// 	defer logger.Warnf("[Event-%s] Sync latest routine exit!", cms.channel)
// 	for {
// 		time.Sleep(SYNC_LATEST_INTERVAL)
// 		height, err := cms.getHeight(cms.channel)
// 		if err != nil {
// 			logger.Errorf("[Event-%s] Get height error: %s", cms.channel, err)
// 			continue
// 		}
// 		logger.Debugf("[Event-%s] syncLatest: latest block hight is: %d,sdk latest is: %d", cms.channel, height, cms.latest)
// 		cms.Lock()
// 		if (height-1) >= 0 && cms.latest < (height-1) {
// 			logger.Debugf("[Event-%s] Get latest block: %d", cms.channel, height-1)
// 			go cms.seekBlock(cms.channel, height-1)
// 		}
// 		cms.Unlock()
// 	}
// }
//
// func (cms *channelMissingStore) SetMissingMap(m map[uint64]int) {
// 	cms.Lock()
// 	defer cms.Unlock()
// 	for k, _ := range m {
// 		cms.missingMap[k] = &missingBlock{
// 			number: k,
// 			status: seeking,
// 			heaven: cms.lifeManager,
// 		}
// 	}
// }
//
// func (cms *channelMissingStore) GetSnapshot() *Serialization {
// 	cms.Lock()
// 	defer cms.Unlock()
// 	snapShot := &Serialization{
// 		Latest:       cms.latest,
// 		MissingBlock: make(map[uint64]int),
// 	}
//
// 	for num, mB := range cms.missingMap {
// 		snapShot.MissingBlock[num] = int(mB.status)
// 	}
// 	return snapShot
// }
