/*
Copyright IBM Corp. 2016 All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

		 http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package ledgerconfig

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/hyperledger/fabric/core/config"
	"github.com/spf13/viper"
)

//IsCouchDBEnabled exposes the useCouchDB variable
func IsCouchDBEnabled() bool {
	stateDatabase := viper.GetString("ledger.state.stateDatabase")
	if stateDatabase == "CouchDB" {
		return true
	}
	return false
}

const confPeerFileSystemPath = "peer.fileSystemPath"
const confLedgersData = "ledgersData"
const confLedgerProvider = "ledgerProvider"
const confStateleveldb = "stateLeveldb"
const confHistoryLeveldb = "historyLeveldb"
const confPvtWritesetStore = "pvtWritesetStore"
const confChains = "chains"
const confPvtdataStore = "pvtdataStore"
const confQueryLimit = "ledger.state.couchDBConfig.queryLimit"
const confEnableHistoryDatabase = "ledger.history.enableHistoryDatabase"
const confMaxBatchSize = "ledger.state.couchDBConfig.maxBatchUpdateSize"
const confAutoWarmIndexes = "ledger.state.couchDBConfig.autoWarmIndexes"
const confWarmIndexesAfterNBlocks = "ledger.state.couchDBConfig.warmIndexesAfterNBlocks"

// GetRootPath returns the filesystem path.
// All ledger related contents are expected to be stored under this path
func GetRootPath() string {
	sysPath := config.GetPath(confPeerFileSystemPath)
	return filepath.Join(sysPath, confLedgersData)
}

// GetLedgerProviderPath returns the filesystem path for storing ledger ledgerProvider contents
func GetLedgerProviderPath() string {
	return filepath.Join(GetRootPath(), confLedgerProvider)
}

// GetStateLevelDBPath returns the filesystem path that is used to maintain the state level db
func GetStateLevelDBPath() string {
	return filepath.Join(GetRootPath(), confStateleveldb)
}

// GetHistoryLevelDBPath returns the filesystem path that is used to maintain the history level db
func GetHistoryLevelDBPath() string {
	return filepath.Join(GetRootPath(), confHistoryLeveldb)
}

// GetPvtWritesetStorePath returns the filesystem path that is used for permanent storage of privare write-sets
func GetPvtWritesetStorePath() string {
	return filepath.Join(GetRootPath(), confPvtWritesetStore)
}

// GetBlockStorePath returns the filesystem path that is used for the chain block stores
func GetBlockStorePath() string {
	return filepath.Join(GetRootPath(), confChains)
}

// GetPvtdataStorePath returns the filesystem path that is used for permanent storage of private write-sets
func GetPvtdataStorePath() string {
	return filepath.Join(GetRootPath(), confPvtdataStore)
}

// GetMaxBlockfileSize returns maximum size of the block file
func GetMaxBlockfileSize() int {
	return 64 * 1024 * 1024
}

//GetQueryLimit exposes the queryLimit variable
func GetQueryLimit() int {
	queryLimit := viper.GetInt(confQueryLimit)
	// if queryLimit was unset, default to 10000
	if !viper.IsSet(confQueryLimit) {
		queryLimit = 10000
	}
	return queryLimit
}

//GetMaxBatchUpdateSize exposes the maxBatchUpdateSize variable
func GetMaxBatchUpdateSize() int {
	maxBatchUpdateSize := viper.GetInt(confMaxBatchSize)
	// if maxBatchUpdateSize was unset, default to 500
	if !viper.IsSet(confMaxBatchSize) {
		maxBatchUpdateSize = 500
	}
	return maxBatchUpdateSize
}

//IsHistoryDBEnabled exposes the historyDatabase variable
func IsHistoryDBEnabled() bool {
	return viper.GetBool(confEnableHistoryDatabase)
}

// IsQueryReadsHashingEnabled enables or disables computing of hash
// of range query results for phantom item validation
func IsQueryReadsHashingEnabled() bool {
	return true
}

// GetMaxDegreeQueryReadsHashing return the maximum degree of the merkle tree for hashes of
// of range query results for phantom item validation
// For more details - see description in kvledger/txmgmt/rwset/query_results_helper.go
func GetMaxDegreeQueryReadsHashing() uint32 {
	return 50
}

//IsAutoWarmIndexesEnabled exposes the autoWarmIndexes variable
func IsAutoWarmIndexesEnabled() bool {
	//Return the value set in core.yaml, if not set, the return true
	if viper.IsSet(confAutoWarmIndexes) {
		return viper.GetBool(confAutoWarmIndexes)
	}
	return true

}

//GetWarmIndexesAfterNBlocks exposes the warmIndexesAfterNBlocks variable
func GetWarmIndexesAfterNBlocks() int {
	warmAfterNBlocks := viper.GetInt(confWarmIndexesAfterNBlocks)
	// if warmIndexesAfterNBlocks was unset, default to 1
	if !viper.IsSet(confWarmIndexesAfterNBlocks) {
		warmAfterNBlocks = 1
	}
	return warmAfterNBlocks
}

func IsDataDumpEnabled() bool {
	return viper.GetBool("ledger.dataDump.enabled")
}

func GetDataDumpPath() string {
	sysPath := config.GetPath("ledger.dataDump.dumpDir")
	return sysPath
}
func GetDataLoadPath() string {
	sysPath := config.GetPath("ledger.dataDump.loadDir")
	return sysPath
}

func GetDataDumpFileLimit() int {
	maxFileLimit := viper.GetInt("ledger.dataDump.maxFileLimit")
	if !viper.IsSet("ledger.dataDump.maxFileLimit") || maxFileLimit <= 0 {
		maxFileLimit = 100000
	}
	return maxFileLimit
}

func GetDataLoadRetryTimes() int {
	retryTimes := viper.GetInt("ledger.dataDump.loadRetryTimes")
	if !viper.IsSet("ledger.dataDump.loadRetryTimes") || retryTimes <= 0 {
		retryTimes = 3
	}
	return retryTimes
}

func GetDataDumpCron() []string {
	data := viper.Get("ledger.dataDump.dumpCron")
	if interfaceList, ok := data.([]interface{}); ok {
		var result []string
		for _, inter := range interfaceList {
			if str, ok := inter.(string); ok {
				result = append(result, str)
			}
		}
		return result
	}
	if strList, ok := data.([]string); ok {
		return strList
	}
	if str, ok := data.(string); ok {
		str = strings.Replace(str, "[", "", 1)
		str = strings.Replace(str, "]", "", 1)
		return strings.Split(str, ",")
	}
	return nil
}

func GetDataDumpInterval() time.Duration {
	return viper.GetDuration("ledger.dataDump.dumpInterval")
}
