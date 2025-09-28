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

func startOrderingPhaseA(i int) {
	type shuffleBuf struct {
		sync.RWMutex
		counter int
		entries map[int]Entry
	}
	shuffle := shuffleBuf{
		entries: make(map[int]Entry),
	}

	cycle := 0

	// 共通：ブロック生成 → スナップショット登録 → ブロードキャスト
	makeAndBroadcast := func(sh *shuffleBuf) {
		sh.RLock()
		if sh.counter == 0 {
			sh.RUnlock()
			return
		}
		// コピーを取ってからロックを解放（ブロードキャスト中に参照しない）
		localEntries := make(map[int]Entry, sh.counter)
		for k := 0; k < sh.counter; k++ {
			if e, ok := sh.entries[k]; ok {
				localEntries[k] = e
			}
		}
		sh.RUnlock()

		// blockchainId は従来踏襲（ServerID）
		var blockchainId = ServerID

		serializedEntries, err := serialization(localEntries)
		if err != nil {
			log.Errorf("serialization failed: %v", err)
			return
		}

		newBlockId := getLogIndex()
		orderingBoothID := getBoothID()

		postEntry := ProposerOPAEntry{
			Booth:        booMgr.b[orderingBoothID],
			BlockchainId: blockchainId,
			BlockId:      newBlockId,
			Entries:      localEntries,
			Hash:         getDigest(serializedEntries),
		}

		nowTime := time.Now().UnixMilli()
		log.Infof("start ordering of block %d, Timestamp: %d, Booth: %v",
			newBlockId, nowTime, booMgr.b[orderingBoothID].Indices)

		if PerfMetres {
			if newBlockId%LatMetreInterval == 0 {
				metre.recordStartTime(newBlockId)
			}
		}

		incrementLogIndex()

		blockOrderFrag := blockSnapshot{
			hash:    postEntry.Hash,
			entries: localEntries,
			sigs:    [][]byte{},
			booth:   booMgr.b[orderingBoothID],
		}
		blockCommitFrag := blockSnapshot{
			hash:    postEntry.Hash,
			entries: localEntries,
			sigs:    [][]byte{},
			booth:   booMgr.b[orderingBoothID],
		}

		ordSnapshot.Lock()
		if _, ok := ordSnapshot.m[postEntry.BlockchainId]; !ok {
			ordSnapshot.m[postEntry.BlockchainId] = make(map[int64]*blockSnapshot)
		}
		ordSnapshot.m[postEntry.BlockchainId][postEntry.BlockId] = &blockOrderFrag
		ordSnapshot.Unlock()

		cmtSnapshot.Lock()
		if _, ok := cmtSnapshot.m[postEntry.BlockchainId]; !ok {
			cmtSnapshot.m[postEntry.BlockchainId] = make(map[int64]*blockSnapshot)
		}
		cmtSnapshot.m[postEntry.BlockchainId][postEntry.BlockId] = &blockCommitFrag
		cmtSnapshot.Unlock()

		// 次バッチに備えてクリア（共有バッファ側）
		sh.Lock()
		sh.counter = 0
		sh.entries = make(map[int]Entry)
		sh.Unlock()

		log.Infof("end ordering phase_a_pro of block %d, Timestamp: %d",
			postEntry.BlockId, time.Now().UnixMilli())

		if EvaluateComPossibilityFlag {
			broadcastToBoothWithComCheck(postEntry, OPA, orderingBoothID)
		} else {
			broadcastToBooth(postEntry, OPA, orderingBoothID)
		}

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

	// ===== モード切替 =====
	if UsePeriodicOrdering {
		// 時間ドリブン（Tick 到来でのみ 1 ブロック）
		log.Infof("[ordering] mode=periodic (time-driven), period=%dms", OrderingPeriodMs)

		ticker := time.NewTicker(time.Duration(OrderingPeriodMs) * time.Millisecond)
		defer ticker.Stop()

		for {
			cycle++

			// ---- バックプレッシャー：BatchSize 以上なら入力を止める（Tick まで）----
			inCh := requestQueue[i]
			shuffle.RLock()
			bufCount := shuffle.counter
			shuffle.RUnlock()
			if bufCount >= BatchSize {
				inCh = nil // select の受信分岐を無効化 → 送信側がブロック
			}

			select {
			case m, ok := <-inCh:
				if !ok {
					log.Infof("requestQueue closed, quitting leader service (server %d)", ServerID)
					return
				}
				// 受信時は「溜めるだけ」。BatchSize 到達でも切らない。
				entry := Entry{TimeStamp: m.Timestamp, Tx: m.Transaction}
				shuffle.Lock()
				idx := shuffle.counter
				shuffle.entries[idx] = entry
				shuffle.counter++
				shuffle.Unlock()

			case <-ticker.C:
				// Tick 到来時だけ 1 ブロック生成。
				shuffle.RLock()
				cnt := shuffle.counter
				shuffle.RUnlock()
				if cnt == 0 {
					continue
				}

				// 取り出し数：min(counter, BatchSize)
				takeN := BatchSize
				if cnt < BatchSize {
					takeN = cnt
				}

				// batch/rest を作成（順序保証が必要なら別途キュー化や key sort を検討）
				batch := make(map[int]Entry, takeN)
				rest := make(map[int]Entry, cnt-takeN)

				shuffle.RLock()
				for k := 0; k < cnt; k++ {
					e, ok := shuffle.entries[k]
					if !ok {
						continue
					}
					if k < takeN {
						batch[len(batch)] = e
					} else {
						rest[len(rest)] = e
					}
				}
				shuffle.RUnlock()

				// ---- 1 本だけ生成：一時的に buffer を batch に差し替えて makeAndBroadcast ----
				shuffle.Lock()
				shuffle.entries = batch
				shuffle.counter = takeN
				shuffle.Unlock()

				makeAndBroadcast(&shuffle)

				// 余りを復帰
				shuffle.Lock()
				shuffle.entries = rest
				shuffle.counter = len(rest)
				shuffle.Unlock()
			}
		}
	} else {
		// 従来：BatchSize 到達で即ブロック化
		log.Infof("[ordering] mode=current (batch-size driven)")

		for {
			cycle++
			m, ok := <-requestQueue[i]
			if !ok {
				log.Infof("requestQueue closed, quitting leader service (server %d)", ServerID)
				return
			}

			entry := Entry{TimeStamp: m.Timestamp, Tx: m.Transaction}
			shuffle.Lock()
			idx := shuffle.counter
			shuffle.entries[idx] = entry
			shuffle.counter++
			shouldCut := shuffle.counter >= BatchSize
			shuffle.Unlock()

			if !shouldCut {
				continue
			}
			// BatchSize 到達で即ブロック化
			makeAndBroadcast(&shuffle)
		}
	}
}

// input->ValidatorOPAReply
func asyncHandleOBReply(m *ValidatorOPAReply, sid ServerId) {

	// log.Infof("start asyncHandleOBReply")

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

	log.Infof("end ordering phase_a_vali of block %d,Timestamp: %d", m.BlockId, time.Now().UnixMilli())

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
	if EvaluateComPossibilityFlag {
		broadcastToBoothWithComCheck(orderEntry, OPB, currBooth.ID)
	} else {
		broadcastToBooth(orderEntry, OPB, currBooth.ID)
	}

	nowTime := time.Now().UnixMilli()
	log.Infof("end ordering of block %d, Timestamp: %d", m.BlockId, nowTime)
}
