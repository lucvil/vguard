package main

/*
The implementation of V-Guard follows a "cache more, lock less" policy. This design
reduces lock overhead and contention while storing more intermediate results.
Intermediate consensus information for data batches are stored separately in the
ordering and consensus phases.
*/

import (
	"encoding/hex"
	"sync"
	"time"
)

// ordSnapshot stores consensus information for each block in the ordering phase
// the map stores <blockID, blockSnapshot>
var ordSnapshot = struct {
	m map[int]map[int64]*blockSnapshot
	sync.RWMutex
}{m: make(map[int]map[int64]*blockSnapshot)}

// cmtSnapshot stores consensus information for each block in the consensus phase
// the map stores <blockID, blockSnapshot>
var cmtSnapshot = struct {
	m map[int]map[int64]*blockSnapshot
	sync.RWMutex
}{m: make(map[int]map[int64]*blockSnapshot)}

type blockSnapshot struct {
	sync.RWMutex
	// The hash of the block
	hash []byte
	// The data entries
	entries map[int]Entry
	// The signatures collected from validators to be converted to a threshold signature
	sigs [][]byte
	// rcvSig is the threshold signature of this block
	tSig []byte
	// The booth of this block
	booth Booth
}

var vgTxMeta = struct {
	sync.RWMutex
	sigs     map[int]map[int][][]byte // map<blockchainId, map<rangeId, sigs[]>>
	hash     map[int]map[int][]byte
	blockIDs map[int]map[int][]int64 // map<blockchainId, map<rangeId, []blockIDs>>
}{
	sigs:     make(map[int]map[int][][]byte),
	hash:     make(map[int]map[int][]byte),
	blockIDs: make(map[int]map[int][]int64),
}

var vgTxData = struct {
	sync.RWMutex
	tx  map[int]map[int]map[string][][]Entry // map<blockchainId, map<consInstID, map<orderingBooth, []entry>>>
	boo map[int]map[int]Booth                //map<blockchainId, map<consInstID, Booth>>
}{
	tx:  make(map[int]map[int]map[string][][]Entry),
	boo: make(map[int]map[int]Booth),
}

func storeVgTx(consInstID int, blockchainId int) {
	vgTxData.RLock()
	ordBoo := vgTxData.tx[blockchainId][consInstID]
	cmtBoo := vgTxData.boo[blockchainId][consInstID]
	vgTxData.RUnlock()

	nowTime := time.Now().UnixMilli()

	log.Infof("VGTX %d in Cmt Booth: %v | total # of tx: %d, Time: %d", consInstID, cmtBoo.Indices, vgrec.GetLastIdx()*BatchSize, nowTime)

	for key, chunk := range ordBoo { //map<boo, [][]entries>
		log.Infof("ordering booth: %v | len(ordBoo[%v]): %v", key, key, len(chunk))
		for _, entries := range chunk {
			for _, e := range entries {
				log.Infof("ts: %v; tx: %v", e.TimeStamp, hex.EncodeToString(e.Tx))
			}
		}
	}
}
