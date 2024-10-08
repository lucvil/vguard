package main

import (
	"encoding/hex"
	"sync"
	"time"
)

var vgrec IDRecorder

type IDRecorder struct {
	sync.RWMutex
	blockIDs []int64
	lastIdx  int
}

func (r *IDRecorder) Add(id int64) {
	r.Lock()
	r.blockIDs = append(r.blockIDs, id)
	r.Unlock()
}

func (r *IDRecorder) GetIDRange() []int64 {
	r.Lock()
	defer r.Unlock()
	curIdx := len(r.blockIDs)
	var blockIDs []int64

	for i := r.lastIdx; i < curIdx; i++ {
		blockIDs = append(blockIDs, r.blockIDs[i])
	}

	r.lastIdx = curIdx
	return blockIDs
}

func (r *IDRecorder) GetLastIdx() int {
	return r.lastIdx
}

//func (r *IDRecorder) GetLastConsPos() int {
//	return r.lastIdx
//}

//func (r *IDRecorder) RecordConsPos()  {
//	defer r.RUnlock()
//	r.RLock()
//	r.lastIdx = len(r.blockIDs)
//}

// collect order and make entry
func startOrderingPhaseA(i int) {

	shuffle := struct {
		sync.RWMutex
		counter int
		entries map[int]Entry
	}{
		counter: 0,
		entries: make(map[int]Entry)}

	cycle := 0

	for {
		cycle++
		m, ok := <-requestQueue[i]

		if !ok {
			log.Infof("requestQueue closed, quiting leader service (server %d)", ServerID)
			return
		}

		entry := Entry{
			TimeStamp: m.Timestamp,
			Tx:        m.Transaction,
		}

		shuffle.entries[shuffle.counter] = entry

		shuffle.counter++
		if shuffle.counter < BatchSize {
			continue
		}

		// shuffle.counter >= BatchSize -> do below code

		//blockchainIdは今の所ServerIDと一緒にする、proposerの変更はいまのところ考えない
		var blockchainId = ServerID

		serializedEntries, err := serialization(shuffle.entries)
		if err != nil {
			log.Errorf("serialization failed, err: %v", err)
			return
		}

		newBlockId := getLogIndex()

		orderingBoothID := getBoothID()

		// create date for order phase a
		postEntry := ProposerOPAEntry{
			Booth:        booMgr.b[orderingBoothID],
			BlockchainId: blockchainId,
			BlockId:      newBlockId,
			Entries:      shuffle.entries,
			Hash:         getDigest(serializedEntries),
		}

		//start time
		nowTime := time.Now().UnixMilli()
		log.Infof("start ordering of block %d, Timestamp: %d", newBlockId, nowTime)

		//peformance metre
		if PerfMetres {
			if newBlockId%LatMetreInterval == 0 {
				metre.recordStartTime(newBlockId)
			}
		}

		incrementLogIndex()
		//booth情報の更新コードを入れる

		blockOrderFrag := blockSnapshot{
			hash:    postEntry.Hash,
			entries: shuffle.entries,
			sigs:    [][]byte{},
			booth:   booMgr.b[orderingBoothID],
		}

		blockCommitFrag := blockSnapshot{
			hash:    postEntry.Hash,
			entries: shuffle.entries,
			sigs:    [][]byte{},
			booth:   booMgr.b[orderingBoothID],
		}

		ordSnapshot.Lock()
		ordSnapshot.m[postEntry.BlockchainId][postEntry.BlockId] = &blockOrderFrag
		ordSnapshot.Unlock()

		cmtSnapshot.Lock()
		cmtSnapshot.m[postEntry.BlockchainId][postEntry.BlockId] = &blockCommitFrag
		cmtSnapshot.Unlock()

		//clear shuffle variables
		shuffle.counter = 0
		shuffle.entries = make(map[int]Entry)

		//broadcast
		broadcastToBooth(postEntry, OPA, orderingBoothID)

		if YieldCycle != 0 {
			if cycle%YieldCycle == 0 {
				time.Sleep(time.Duration(EasingDuration) * time.Millisecond)
			}
		}

		log.Debugf("new PostEntryBlock broadcastToBooth -> blk_id: %d | blk_hash: %s",
			postEntry.BlockId, hex.EncodeToString(postEntry.Hash))

		nowTime = time.Now().UnixMilli()
		log.Infof("broadcast ordering of block %d, Timestamp: %d", newBlockId, nowTime)
	}
}

// input->ValidatorOPAReply
func asyncHandleOBReply(m *ValidatorOPAReply, sid ServerId) {

	log.Infof("start asyncHandleOBReply")

	ordSnapshot.RLock()
	blockOrderFrag, ok := ordSnapshot.m[m.BlockchainId][m.BlockId]
	ordSnapshot.RUnlock()

	if !ok {
		log.Debugf("%s | no info of [block:%v] in cache; consensus may already reached | sid: %v", cmdPhase[OPB], m.BlockId, sid)
		return
	}

	currBooth := blockOrderFrag.booth

	blockOrderFrag.Lock()

	//ブロックの注文情報のロックを取得し、現在のシグネチャ数を確認します。
	//シグネチャの数が既に閾値に達していれば、そのブロックはすでに注文済みであるため処理を終了します。
	//シグネチャの数が閾値未満であれば、新しいシグネチャ（m.ParSig）を追加します。
	indicator := len(blockOrderFrag.sigs)

	threshold := getThreshold(len(currBooth.Indices))

	// log.Infof("received OPA reply from %v for block %v, threshold: %d", sid, m.BlockId, threshold)

	if indicator == threshold {
		blockOrderFrag.Unlock()

		log.Debugf("%s | Block %d already ordered | indicator: %v | Threshold: %v | sid: %v",
			cmdPhase[OPB], m.BlockId, indicator, threshold, sid)
		return
	}

	indicator++
	aggregatedSigs := append(blockOrderFrag.sigs, m.ParSig)
	blockOrderFrag.sigs = aggregatedSigs

	blockOrderFrag.Unlock()

	if indicator < threshold {
		log.Debugf("%s | insufficient votes for ordering | blockId: %v | indicator: %v | sid: %v",
			cmdPhase[OPB], m.BlockId, indicator, sid)
		return
	}

	if indicator > threshold {
		log.Debugf("%s | block %v already broadcastToBooth for ordering | sid: %v", cmdPhase[OPB], m.BlockId, sid)
		return
	}

	// log.Infof("collect enough OBReply")

	///複数の署名断片から完全なデジタル署名を復元するための関数s
	publicPolyPOB, _ := fetchKeysByBoothId(threshold, ServerID, currBooth.ID, m.BlockchainId)
	thresholdSig, err := PenRecovery(aggregatedSigs, &blockOrderFrag.hash, publicPolyPOB, len(currBooth.Indices))
	if err != nil {
		log.Errorf("%s | blockId: %v | PenRecovery failed | len(sigShares): %v | booth: %v| error: %v",
			cmdPhase[OPB], m.BlockId, len(aggregatedSigs), currBooth, err)
		return
	}

	// log.Infof("fetch key and make thresholdSig")

	orderEntry := ProposerOPBEntry{
		BlockchainId: m.BlockchainId,
		Booth:        currBooth,
		BlockId:      m.BlockId,
		CombSig:      thresholdSig,
		//Entries: blockOrderFrag.entries, ??
		Hash: blockOrderFrag.hash,
	}

	vgrec.Add(m.BlockId)

	//peformance metre

	if PerfMetres {
		timeNow := time.Now().UTC().String()
		if m.BlockId%LatMetreInterval == 0 {
			metre.recordOrderTime(m.BlockId)
		}

		if m.BlockId%100 == 0 {
			if m.BlockId == 0 {
				log.Infof("<METRE> ordering block %d ordered (tx: %d) at %s", m.BlockId, BatchSize, timeNow)
			} else {
				log.Infof("<METRE> ordering block %d ordered (tx: %d) at %s", m.BlockId, m.BlockId*int64(BatchSize), timeNow)
			}
		}
	}

	cmtSnapshot.Lock()
	cmtSnapshot.m[m.BlockchainId][m.BlockId].tSig = thresholdSig
	cmtSnapshot.Unlock()

	// log.Infof("finish OBReply")

	broadcastToBooth(orderEntry, OPB, currBooth.ID)

	nowTime := time.Now().UnixMilli()
	log.Infof("end ordering of block %d, Timestamp: %d", m.BlockId, nowTime)
}
