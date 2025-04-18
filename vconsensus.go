package main

import (
	"time"
)

func startConsensusPhaseA() {
	consInstID := 0
	waited := 0

	for {
		commitBoothID := getBoothID()
		var blockchainId = ServerID

		// switch BoothMode {
		// case BoothModeOCSB:

		// case BoothModeOCDBWOP:
		// 	commitBoothID = BoothIDOfModeOCDBWOP
		// case BoothModeOCDBNOP:
		// 	commitBoothID = BoothIDOfModeOCDBNOP
		// }

		commitBooth := booMgr.b[commitBoothID]
		time.Sleep(time.Duration(ConsInterval) * time.Millisecond)

		// start time
		nowTime := time.Now().UnixMilli()

		blockIDRange := vgrec.GetIDRange()

		if blockIDRange == nil {
			log.Warnf("Waiting for new ordered blocks [consInstID:%v | LastOrdIdx: %v]; consensus has waited for %v second(s)",
				consInstID, vgrec.GetLastIdx(), waited)

			time.Sleep(1 * time.Second)

			waited++
			if waited < ConsWaitingSec {
				continue
			}

			log.Warnf("Terminating current VGuard instance")
			vgInst.Done()
			return
		}

		// log.Infof("start consensus of consInstId: %d, blockIDRange: %d,Timestamp: %d", consInstID, blockIDRange, nowTime)
		log.Infof("start consensus of consInstId: %d,Timestamp: %d, lenBlockRange: %d, Booth: %v", consInstID, nowTime, len(blockIDRange), commitBooth.Indices)

		blockHashesInRange := make(map[int64][]byte)

		vgTxData.Lock()
		if _, ok := vgTxData.tx[blockchainId]; !ok {
			vgTxData.tx[blockchainId] = make(map[int]map[string][][]Entry)
		}
		if _, ok := vgTxData.boo[blockchainId]; !ok {
			vgTxData.boo[blockchainId] = make(map[int]Booth)
		}
		vgTxData.tx[blockchainId][consInstID] = make(map[string][][]Entry)
		vgTxData.Unlock()

		var newMembers []int
		var resentOPBEntries []ProposerOPBEntry

		for _, blockID := range blockIDRange {
			var blockEntries []Entry

			cmtSnapshot.Lock()
			blockCmtFrag, ok := cmtSnapshot.m[blockchainId][blockID]
			cmtSnapshot.Unlock()

			if !ok {
				log.Errorf("cmtSnapshot.h[%d] not exists; ; continue", blockID)
				continue
			}

			blockHashesInRange[blockID] = blockCmtFrag.hash

			for _, entry := range blockCmtFrag.entries {
				blockEntries = append(blockEntries, entry)
			}

			ordBoo := blockCmtFrag.booth

			newMemberFlag := false

			for _, cmtMember := range commitBooth.Indices {
				if !BooIndices(ordBoo.Indices).Contain(cmtMember) {
					newMemberFlag = true
					if !BooIndices(newMembers).Contain(cmtMember) {
						newMembers = append(newMembers, cmtMember)
					}
					log.Debugf("%s | %v is the new member in CMT-BOO: %v | ORD-BOO: %v | BlockID: %v",
						cmdPhase[CPA], cmtMember, commitBooth.Indices, ordBoo.Indices, blockID)
				}
			}

			if newMemberFlag {
				resentOPBEntries = append(resentOPBEntries, ProposerOPBEntry{
					BlockchainId: blockchainId,
					Booth:        ordBoo,
					BlockId:      blockID,
					CombSig:      blockCmtFrag.tSig,
					Entries:      blockCmtFrag.entries,
					Hash:         blockCmtFrag.hash,
				})
				if blockCmtFrag.tSig == nil {
					log.Errorf("CombSig is nil ! len of Entries: %v", len(blockCmtFrag.entries))
					log.Errorf("blockCmtFrag: %+v", blockCmtFrag)
				}
			}

			boo, err := ordBoo.String()
			if err != nil {
				log.Error(err)
				return
			}

			vgTxData.Lock()
			if _, ok := vgTxData.tx[blockchainId][consInstID][boo]; ok {
				vgTxData.tx[blockchainId][consInstID][boo] = append(vgTxData.tx[blockchainId][consInstID][boo], blockEntries)
			} else {
				vgTxData.tx[blockchainId][consInstID][boo] = [][]Entry{blockEntries}
			}

			vgTxData.boo[blockchainId][consInstID] = commitBooth
			vgTxData.Unlock()
		}
		nowTime = time.Now().UnixMilli()

		serialized, err := serialization(blockHashesInRange)

		if err != nil {
			log.Error(err)
			break
		}

		totalHash := getDigest(serialized)

		entryCA := ProposerCPAEntry{
			PrevOPBEntries: nil,
			BlockchainId:   blockchainId,
			Booth:          commitBooth,
			BIDs:           blockIDRange,
			ConsInstID:     consInstID,
			RangeHash:      blockHashesInRange,
			TotalHash:      totalHash,
		}

		// store the hash and blockIDRange before sending to vaidators
		vgTxMeta.Lock()
		if _, ok := vgTxMeta.sigs[blockchainId]; !ok {
			vgTxMeta.sigs[blockchainId] = make(map[int][][]byte)
		}
		if _, ok := vgTxMeta.hash[blockchainId]; !ok {
			vgTxMeta.hash[blockchainId] = make(map[int][]byte)
		}
		if _, ok := vgTxMeta.blockIDs[blockchainId]; !ok {
			vgTxMeta.blockIDs[blockchainId] = make(map[int][]int64)
		}
		vgTxMeta.hash[blockchainId][consInstID] = totalHash
		vgTxMeta.blockIDs[blockchainId][consInstID] = blockIDRange
		vgTxMeta.Unlock()

		log.Infof("end consensus phase_a_pro of consInstId: %d,Timestamp: %d", consInstID, time.Now().UnixMilli())

		if newMembers == nil {
			if EvaluateComPossibilityFlag {
				broadcastToBoothWithComCheck(entryCA, CPA, commitBoothID)
			} else {
				broadcastToBooth(entryCA, CPA, commitBoothID)
			}
		} else {
			newEntryCA := entryCA
			newEntryCA.SetPrevOPBEntries(resentOPBEntries)
			log.Debugf("%s | sending newEntry CA to %v | len(resentOPBEntries): %v", cmdPhase[CPA], newMembers, len(resentOPBEntries))
			if EvaluateComPossibilityFlag {
				broadcastToNewBoothWithComCheck(entryCA, CPA, commitBoothID, newMembers, newEntryCA)
			} else {
				broadcastToNewBooth(entryCA, CPA, commitBoothID, newMembers, newEntryCA)
			}
		}

		consInstID++
	}
}

func asyncHandleCPAReply(m *ValidatorCPAReply, sid ServerId) {

	// log.Infof("receive cpa reply blockid: %d, validatorId: %d", m.ConsInstID, sid)

	nowTime := time.Now().UnixMilli()

	vgTxMeta.RLock()
	fetchedTotalHash, ok := vgTxMeta.hash[m.BlockchainId][m.ConsInstID]
	partialSig := vgTxMeta.sigs[m.BlockchainId][m.ConsInstID]
	vgTxMeta.RUnlock()

	if !ok {
		log.Debugf("%s | rangId %d is not in vgTxMeta", cmdPhase[CPA], m.ConsInstID)
		return
	}

	vgTxData.RLock()
	residingBooth := vgTxData.boo[m.BlockchainId][m.ConsInstID]
	vgTxData.RUnlock()

	boothSize := len(residingBooth.Indices)
	threshold := getThreshold(len(residingBooth.Indices))

	if len(partialSig) == threshold {
		log.Debugf("%s | Batch already committed| commitIndicator: %v | Threshold: %v | RangeId: %v | sid: %v",
			cmdPhase[CPA], len(partialSig), threshold, m.ConsInstID, sid)
		return
	}

	partialSig = append(partialSig, m.ParSig)

	vgTxMeta.Lock()
	vgTxMeta.sigs[m.BlockchainId][m.ConsInstID] = partialSig
	vgTxMeta.Unlock()

	if len(partialSig) < threshold {
		log.Debugf("%s | insufficient votes | blockId: %d | indicator: %d | sid: %v", cmdPhase[CPA], m.ConsInstID, len(partialSig), sid)
		return
	} else if len(partialSig) > threshold {
		log.Debugf("%s | block %d already broadcastToBooth | indicator: %d | sid: %v", cmdPhase[CPA], m.ConsInstID, len(partialSig), sid)
		return
	}

	log.Infof("end consensus phase_a_vali of consInstId: %d,Timestamp: %d", m.ConsInstID, time.Now().UnixMilli())

	log.Debugf(" ** votes sufficient | rangeId: %v | votes: %d | sid: %v", m.ConsInstID, len(partialSig), sid)

	publicPolyPCA, _ := fetchKeysByBoothId(threshold, ServerID, residingBooth.ID, m.BlockchainId)

	log.Infof("end fetch publickPolyPCA key of consInstId: %d,Timestamp: %d", m.ConsInstID, time.Now().UnixMilli())
	recoveredSig, err := PenRecovery(partialSig, &fetchedTotalHash, publicPolyPCA, boothSize)

	log.Infof("end recover Sig of consInstId: %d,Timestamp: %d", m.ConsInstID, time.Now().UnixMilli())
	if err != nil {
		log.Errorf("partialSig: %v, fetchedTotalHash; %v", partialSig, &fetchedTotalHash)

		log.Errorf("%s | PenRecovery failed | len(sigShares): %d | threshold: %d | publicPolyPCA: %+v | m.BlockchainId: %d | residingBooth.ID: %d | Block.ID: %d |error: %v", cmdPhase[CPA], len(partialSig), threshold, publicPolyPCA, m.BlockchainId, residingBooth.ID, m.ConsInstID, err)
		return
	}

	entryCB := ProposerCPBEntry{
		BlockchainId: m.BlockchainId,
		Booth:        residingBooth,
		ConsInstID:   m.ConsInstID,
		ComSig:       recoveredSig,
		Hash:         fetchedTotalHash,
	}

	if EvaluateComPossibilityFlag {
		broadcastToBoothWithComCheck(entryCB, CPB, residingBooth.ID)
	} else {
		broadcastToBooth(entryCB, CPB, residingBooth.ID)
	}

	nowTime = time.Now().UnixMilli()
	log.Infof("end consensus of consInstId: %d,Timestamp: %d", m.ConsInstID, nowTime)

	// if PerfMetres {
	// 	// storeVgTx(m.ConsInstID)
	// }

	// Future work: garbage collection

	if PerfMetres {
		metre.printConsensusLatency(vgrec.lastIdx)
	}
}
