package main

import (
	"bytes"
	"encoding/gob"
	"encoding/hex"
	"sync"
	"time"
)

var valiConsJobStack = struct {
	sync.RWMutex
	s map[int]map[int]chan int // map<blockchainId, map<ConsInstID, chan 1>>
}{s: make(map[int]map[int]chan int)}

var valiOrdJobStack = struct {
	sync.RWMutex
	s map[int]map[int64]chan int // <OrdInstID, chan 1>
}{s: make(map[int]map[int64]chan int)}

// return signature
func validatingOAEntry(m *ProposerOPAEntry, encoder *gob.Encoder) {
	log.Debugf("%s | ProposerOPBEntry received (BlockID: %d) @ %v", rpyPhase[OPA], m.BlockId, time.Now().UTC().String())

	// log.Infof("serverID: %d,start validating OAEntry blockID: %d", ServerID, m.BlockId)
	//受信したブロックIDが既に使用されているかどうかを確認します。
	//既に使用されていれば、ロックを解除し、警告ログを出力して関数を終了します。
	ordSnapshot.Lock()
	if _, ok := ordSnapshot.m[m.BlockchainId][m.BlockId]; ok {
		ordSnapshot.Unlock()
		log.Warnf("%s | blockID %v already used", rpyPhase[OPA], m.BlockId)
		return
	}

	snapshot := blockSnapshot{
		hash:    m.Hash,
		entries: m.Entries,
		sigs:    nil,
		booth:   m.Booth,
	}

	if _, ok := ordSnapshot.m[m.BlockchainId]; !ok {
		ordSnapshot.m[m.BlockchainId] = make(map[int64]*blockSnapshot)
	}
	ordSnapshot.m[m.BlockchainId][m.BlockId] = &snapshot
	ordSnapshot.Unlock()

	cmtSnapshot.Lock()
	if _, ok := cmtSnapshot.m[m.BlockchainId]; !ok {
		cmtSnapshot.m[m.BlockchainId] = make(map[int64]*blockSnapshot)
	}
	cmtSnapshot.m[m.BlockchainId][m.BlockId] = &blockSnapshot{
		hash:    m.Hash,
		entries: m.Entries,
		tSig:    nil,
		booth:   m.Booth,
	}

	//validation??
	valiOrdJobStack.Lock()
	if _, ok := valiOrdJobStack.s[m.BlockchainId]; !ok {
		valiOrdJobStack.s[m.BlockchainId] = make(map[int64]chan int)
	}
	if s, ok := valiOrdJobStack.s[m.BlockchainId][m.BlockId]; !ok {
		//容量1の整数型バッファ付きチャンネルを作成する式
		s := make(chan int, 1)
		valiOrdJobStack.s[m.BlockchainId][m.BlockId] = s
		valiOrdJobStack.Unlock()
		//valiOrdJobStack.s[m.BlockId]には、チャンネルが格納されているので、そのチャンネルに1を送信する
		s <- 1
	} else {
		valiOrdJobStack.Unlock()
		s <- 1
	}

	cmtSnapshot.Unlock()

	//署名を作成します
	threshold := getThreshold(len(m.Booth.Indices))
	_, privateShareVOA := fetchKeysByBoothId(threshold, ServerID, m.Booth.ID, m.BlockchainId)
	sig, err := PenSign(m.Hash, privateShareVOA)
	if err != nil {
		log.Errorf("%s | PenSign failed, err: %v", rpyPhase[OPA], err)
		return
	}

	postReply := ValidatorOPAReply{
		BlockchainId: m.BlockchainId,
		BlockId:      m.BlockId,
		ParSig:       sig,
	}

	log.Debugf("%s | msg: %v; ps: %v", rpyPhase[OPA], m.BlockId, hex.EncodeToString(sig))

	// log.Infof("serverID: %d,end validating OAEntry blockID: %d", ServerID, m.BlockId)

	if EvaluateComPossibilityFlag {
		blockchainInfo.RLock()
		var recipientProposerId = blockchainInfo.m[postReply.BlockchainId][OPA]
		blockchainInfo.RUnlock()
		dialSendBackWithComCheck(postReply, encoder, OPB, recipientProposerId)
	} else {
		dialSendBack(postReply, encoder, OPA)
	}

	log.Infof("end ordering validate and send back blockId: %d", postReply.BlockId)
}

func validatingOBEntry(m *ProposerOPBEntry, encoder *gob.Encoder) {

	if encoder != nil {
		log.Debugf("%s | ProposerOPBEntry received (BlockID: %d) @ %v", rpyPhase[OPB], m.BlockId, time.Now().UTC().String())
	} else {
		log.Debugf("%s | Sync up -> ProposerOPBEntry received (BlockID: %d) @ %v", rpyPhase[CPA], m.BlockId, time.Now().UTC().String())
	}

	//ブロックのハッシュと組み合わせ署名を検証します。
	threshold := getThreshold(len(m.Booth.Indices))
	publicPolyVOB, _ := fetchKeysByBoothId(threshold, ServerID, m.Booth.ID, m.BlockchainId)
	// if fetchErr != nil {
	// 	log.Errorf("Failed to fetch keys for verification: %v, ServerID :%d, m.Booth.ID: %d, m.BlockchainId: %d", fetchErr, ServerID, m.Booth.ID, m.BlockchainId)
	// 	return
	// }

	err := PenVerify(m.Hash, m.CombSig, publicPolyVOB)
	if err != nil {
		log.Errorf("%v: PenVerify failed | err: %v | BlockID: %v | m.Hash: %v| CombSig: %v",
			rpyPhase[OPB], err, m.BlockId, hex.EncodeToString(m.Hash), hex.EncodeToString(m.CombSig))

		return
	}

	ordSnapshot.RLock()
	_, ok := ordSnapshot.m[m.BlockchainId][m.BlockId]
	ordSnapshot.RUnlock()

	if !ok {
		// It is common that some validators have not seen this message.
		// Consensus requires only 2f+1 servers, in which f of them may
		// not receive the message in the previous phase.
		log.Debugf("%v : block %v not stored in ordSnapshot (size of %v)", rpyPhase[OPB], m.BlockId, len(ordSnapshot.m))

		if encoder == nil {
			cmtSnapshot.Lock()
			if cmtSnapshot.m[m.BlockchainId] == nil {
				cmtSnapshot.m[m.BlockchainId] = make(map[int64]*blockSnapshot)
			}
			cmtSnapshot.m[m.BlockchainId][m.BlockId] = &blockSnapshot{
				hash:    m.Hash,
				entries: m.Entries,
				tSig:    m.CombSig,
				booth:   m.Booth,
			}
			cmtSnapshot.Unlock()
		}
		return
	} else {
		log.Debugf("%s | ordSnapshot fetched for BlockId: %d", rpyPhase[OPB], m.BlockId)
	}

	cmtSnapshot.Lock()
	if _, ok := cmtSnapshot.m[m.BlockchainId][m.BlockId]; !ok {

		valiOrdJobStack.Lock()
		if s, ok := valiOrdJobStack.s[m.BlockchainId][m.BlockId]; !ok {
			s := make(chan int, 1)
			valiOrdJobStack.s[m.BlockchainId][m.BlockId] = s
			valiOrdJobStack.Unlock()
			<-s
		} else {
			valiOrdJobStack.Unlock()
			<-s
		}
	}
	cmtSnapshot.m[m.BlockchainId][m.BlockId].tSig = m.CombSig
	cmtSnapshot.Unlock()

	log.Debugf("block %d ordered", m.BlockId)

	log.Infof("end ordering of block %d, Timestamp: %d, BlockchainId: %d", m.BlockId, time.Now().UnixMilli(), m.BlockchainId)
}

func validatingCAEntry(m *ProposerCPAEntry, encoder *gob.Encoder) {
	// nowTime := time.Now().UnixMilli()
	// log.Infof("start consensus phase_a_val of consInstId: %d,Timestamp: %d", m.ConsInstID, nowTime)

	log.Debugf("%s | ProposerCPAEntry received (RangeId: %d) @ %v", rpyPhase[CPA], m.ConsInstID, time.Now().UTC().String())

	vgTxData.Lock()
	if _, ok := vgTxData.tx[m.BlockchainId]; !ok {
		vgTxData.tx[m.BlockchainId] = make(map[int]map[string][][]Entry)
	}
	vgTxData.tx[m.BlockchainId][m.ConsInstID] = make(map[string][][]Entry)
	vgTxData.Unlock()

	if m.PrevOPBEntries != nil {
		log.Debugf("%s | %v| len(PrevOPBEntries): %v", rpyPhase[CPA], m.ConsInstID, len(m.PrevOPBEntries))

		for _, OPBEntry := range m.PrevOPBEntries {
			validatingOBEntry(&OPBEntry, nil)
		}
	}

	if m.BIDs == nil {
		log.Errorf("%s | ConsInstID: %v | Empty BIDs shouldn't have been transmitted", rpyPhase[CPA], m.ConsInstID)
		return
	}

	for _, blockID := range m.BIDs {
		cmtSnapshot.RLock()
		snapshot, ok := cmtSnapshot.m[m.BlockchainId][blockID]
		cmtSnapshot.RUnlock()

		if !ok {
			log.Infof("%v | cmtSnapshot.h[%v] not found in cache|", rpyPhase[CPA], blockID)
			continue
		}

		if !bytes.Equal(snapshot.hash, m.RangeHash[blockID]) {
			log.Errorf(" block hashes don't match; ConsInstID:%d mapsize:%d; received m.RangeHash[%v]: %v | local: %v",
				m.ConsInstID, len(m.RangeHash), blockID, m.RangeHash[blockID], snapshot.hash)
			return
		}

		var blockEntries []Entry

		for _, entry := range snapshot.entries {
			blockEntries = append(blockEntries, entry)
		}

		boo, err := snapshot.booth.String()
		if err != nil {
			log.Error(err)
			return
		}

		vgTxData.Lock()
		if _, ok := vgTxData.tx[m.BlockchainId][m.ConsInstID][boo]; ok {
			vgTxData.tx[m.BlockchainId][m.ConsInstID][boo] = append(vgTxData.tx[m.BlockchainId][m.ConsInstID][boo], blockEntries)
		} else {
			vgTxData.tx[m.BlockchainId][m.ConsInstID][boo] = [][]Entry{blockEntries}
		}

		if _, ok := vgTxData.boo[m.BlockchainId]; !ok {
			vgTxData.boo[m.BlockchainId] = make(map[int]Booth)
		}
		vgTxData.boo[m.BlockchainId][m.ConsInstID] = m.Booth
		vgTxData.Unlock()
	}

	threshold := getThreshold(len(m.Booth.Indices))
	_, privateShareVCA := fetchKeysByBoothId(threshold, ServerID, m.Booth.ID, m.BlockchainId)
	partialSig, err := PenSign(m.TotalHash, privateShareVCA)
	if err != nil {
		log.Errorf("PenSign failed: %v", err)
		return
	}

	replyCA := ValidatorCPAReply{
		BlockchainId: m.BlockchainId,
		ConsInstID:   m.ConsInstID,
		ParSig:       partialSig,
	}

	vgTxMeta.Lock()
	if _, ok := vgTxMeta.hash[m.BlockchainId]; !ok {
		vgTxMeta.hash[m.BlockchainId] = make(map[int][]byte)
	}
	vgTxMeta.hash[m.BlockchainId][m.ConsInstID] = m.TotalHash
	vgTxMeta.Unlock()

	valiConsJobStack.Lock()
	if _, ok := valiConsJobStack.s[m.BlockchainId]; !ok {
		valiConsJobStack.s[m.BlockchainId] = make(map[int]chan int)
	}

	if s, ok := valiConsJobStack.s[m.BlockchainId][m.ConsInstID]; !ok {
		s := make(chan int, 1)
		valiConsJobStack.s[m.BlockchainId][m.ConsInstID] = s
		valiConsJobStack.Unlock()
		s <- 1
	} else {
		valiConsJobStack.Unlock()
		s <- 1
	}

	if EvaluateComPossibilityFlag {
		blockchainInfo.RLock()
		var recipientProposerId = blockchainInfo.m[replyCA.BlockchainId][CPA]
		blockchainInfo.RUnlock()
		dialSendBackWithComCheck(replyCA, encoder, CPA, recipientProposerId)
	} else {
		dialSendBack(replyCA, encoder, CPA)
	}

	nowTime := time.Now().UnixMilli()
	log.Infof("end consensus phase_a_val of consInstId: %d,Timestamp: %d", m.ConsInstID, nowTime)

}

func validatingCBEntry(m *ProposerCPBEntry, encoder *gob.Encoder) {

	vgTxMeta.RLock()
	_, ok := vgTxMeta.hash[m.BlockchainId][m.ConsInstID]
	vgTxMeta.RUnlock()

	if !ok {
		log.Debugf("%v | vgTxMeta.hash[m.RangeId:%v] not stored in cache|", rpyPhase[CPB], m.ConsInstID)
	}

	// Wait for prior job to be finished first
	valiConsJobStack.Lock()
	if _, ok := valiConsJobStack.s[m.BlockchainId]; !ok {
		valiConsJobStack.s[m.BlockchainId] = make(map[int]chan int)
	}

	if s, ok := valiConsJobStack.s[m.BlockchainId][m.ConsInstID]; !ok {
		s := make(chan int, 1)
		valiConsJobStack.s[m.BlockchainId][m.ConsInstID] = s
		valiConsJobStack.Unlock()
		<-s
	} else {
		valiConsJobStack.Unlock()
		<-s
	}

	threshold := getThreshold(len(m.Booth.Indices))
	publicPolyVCB, _ := fetchKeysByBoothId(threshold, ServerID, m.Booth.ID, m.BlockchainId)
	err := PenVerify(m.Hash, m.ComSig, publicPolyVCB)
	if err != nil {
		log.Errorf("%v | PenVerify failed; err: %v", rpyPhase[CPB], err)
		return
	}

	log.Infof("end consensus of consInstId: %d,Timestamp: %d, BlockchainId: %d", m.ConsInstID, time.Now().UnixMilli(), m.BlockchainId)

	// log
	// storeVgTx(m.ConsInstID)

	//replyCB := ValidatorCPBReply{
	//	RangeId: m.RangeId,
	//	Done:    true,
	//}
	//
	//dialSendBack(replyCB, encoder, CPB)
}
