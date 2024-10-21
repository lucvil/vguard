package main

import (
	"encoding/gob"
	"errors"
	"io"
	"net"
	"reflect"
	"time"
)

// validators' connections:
func runAsValidator() {
	defer proposerLookup.RUnlock()
	proposerLookup.RLock()

	fetchValToProComTimeMap([]ServerId{ServerId(ServerID)})

	for _, coordinatorId := range proposerLookup.m[OPA] {
		registerDialConn(coordinatorId, OPA, ListenerPortOPA)
		registerDialConn(coordinatorId, OPB, ListenerPortOPB)
		registerDialConn(coordinatorId, CPA, ListenerPortOCA)
		registerDialConn(coordinatorId, CPB, ListenerPortOCB)
		registerDialConn(coordinatorId, TIME, ListenerPortTIME)
	}

	log.Debugf("... registerDialConn completed ...")

	for index, _ := range proposerLookup.m[OPA] {
		go receivingOADialMessages(proposerLookup.m[OPA][index])
		go receivingOBDialMessages(proposerLookup.m[OPB][index])
		go receivingCADialMessages(proposerLookup.m[CPA][index])
		go receivingCBDialMessages(proposerLookup.m[CPB][index])
		go receivingTIMEDialMessages(proposerLookup.m[TIME][index])
	}
}

func registerDialConn(coordinatorId ServerId, phaseNumber Phase, portNumber int) {
	coordinatorIp := ServerList[coordinatorId].Ip
	coordinatorListenerPort := ServerList[coordinatorId].Ports[portNumber]
	coordinatorAddress := coordinatorIp + ":" + coordinatorListenerPort

	conn, err := establishDialConn(coordinatorId, coordinatorAddress, int(phaseNumber))
	if err != nil {
		log.Errorf("dialog to coordinator %v failed | error: %v", phaseNumber, err)
		return
	}

	log.Infof("dial conn of Phase %d has established | remote addr: %s", phaseNumber, conn.RemoteAddr().String())

	dialogMgr.Lock()
	dialogMgr.conns[phaseNumber][coordinatorId] = ConnDock{
		SID:  coordinatorId,
		conn: conn,
		enc:  gob.NewEncoder(conn),
		dec:  gob.NewDecoder(conn),
	}
	dialogMgr.Unlock()

	blockchainInfo.Lock()
	if blockchainInfo.m[int(coordinatorId)] == nil {
		blockchainInfo.m[int(coordinatorId)] = make(map[Phase]ServerId)
	}
	blockchainInfo.m[int(coordinatorId)][phaseNumber] = coordinatorId
	blockchainInfo.Unlock()

	log.Infof("dial conn of Phase %d has registered | dialogMgr.conns[phaseNumber: %d][coordinatorId: %d]: localconn: %s, remoteconn: %s",
		phaseNumber, phaseNumber, coordinatorId, dialogMgr.conns[phaseNumber][coordinatorId].conn.LocalAddr().String(),
		dialogMgr.conns[phaseNumber][coordinatorId].conn.RemoteAddr().String())
}

func establishDialConn(coordinatorId ServerId, coordListenerAddr string, phase int) (*net.TCPConn, error) {
	var e error

	coordTCPListenerAddr, err := net.ResolveTCPAddr("tcp4", coordListenerAddr)
	if err != nil {
		panic(err)
	}

	coordinatorIndex := -1
	for i, v := range ProposerList {
		if v == coordinatorId {
			coordinatorIndex = i
			break
		}
	}

	if coordinatorIndex == -1 {
		panic(errors.New("coordinator not found in ProposerList"))
	}

	ServerList[ServerID].RLock()

	var myDialAddr string
	myDialAdrIp := ServerList[ServerID].Ip

	switch phase {
	case OPA:
		myDialAddr = myDialAdrIp + ":" + ServerList[ServerID].Ports[DialPortOPA+coordinatorIndex*NOP*2]
	case OPB:
		myDialAddr = myDialAdrIp + ":" + ServerList[ServerID].Ports[DialPortOPB+coordinatorIndex*NOP*2]
	case CPA:
		myDialAddr = myDialAdrIp + ":" + ServerList[ServerID].Ports[DialPortCPA+coordinatorIndex*NOP*2]
	case CPB:
		myDialAddr = myDialAdrIp + ":" + ServerList[ServerID].Ports[DialPortCPB+coordinatorIndex*NOP*2]
	case TIME:
		myDialAddr = myDialAdrIp + ":" + ServerList[ServerID].Ports[DialPortTIME+coordinatorIndex*NOP*2]
	default:
		panic(errors.New("wrong phase name"))
	}

	ServerList[ServerID].RUnlock()

	myTCPDialAddr, err := net.ResolveTCPAddr("tcp4", myDialAddr)

	if err != nil {
		panic(err)
	}

	maxTry := 10
	for i := 0; i < maxTry; i++ {
		conn, err := net.DialTCP("tcp4", myTCPDialAddr, coordTCPListenerAddr)

		if err != nil {
			log.Errorf("Dial Leader failed | err: %v | maxTry: %v | retry: %vth\n", err, maxTry, i)
			time.Sleep(1 * time.Second)
			e = err
			continue
		}
		return conn, nil
	}

	return nil, e
}

func receivingOADialMessages(coordinatorId ServerId) {
	dialogMgr.RLock()
	postPhaseDialogInfo := dialogMgr.conns[OPA][coordinatorId]
	orderPhaseDialogInfo := dialogMgr.conns[OPB][coordinatorId]
	dialogMgr.RUnlock()

	for {
		var m ProposerOPAEntry

		err := postPhaseDialogInfo.dec.Decode(&m)

		log.Infof("start ordering blockId: %v", m.BlockId)

		if err == io.EOF {
			nowTime := time.Now().UnixMilli()
			log.Errorf("%v | coordinator closed connection | err: %v, time=%d", rpyPhase[OPA], err, nowTime)
			log.Warnf("Lost connection with the proposer (S%v); quitting program", postPhaseDialogInfo.SID)
			vgInst.Done()
			break
		}

		if err != nil {
			log.Errorf("Gob Decode Err: %v", err)
			continue
		}

		//メッセージとエンコーダー（orderPhaseDialogInfo.enc）が渡されます
		go validatingOAEntry(&m, orderPhaseDialogInfo.enc)
	}
}

func receivingOBDialMessages(coordinatorId ServerId) {
	dialogMgr.RLock()
	orderPhaseDialogInfo := dialogMgr.conns[OPB][coordinatorId]
	commitPhaseDialogInfo := dialogMgr.conns[CPA][coordinatorId]
	dialogMgr.RUnlock()

	for {
		var m ProposerOPBEntry

		err := orderPhaseDialogInfo.dec.Decode(&m)

		if err == io.EOF {
			nowTime := time.Now().UnixMilli()
			log.Errorf("%s | coordinator closed connection | err: %v, time=%d", rpyPhase[OPB], err, nowTime)
			break
		}

		// if err != nil {
		// 	log.Errorf("%s | gob Decode Err: %v, ServerId: %d", rpyPhase[OPB], err, int(coordinatorId))
		// 	log.Errorf("The type of the message is:", reflect.TypeOf())
		// 	continue
		// }

		if err != nil {
			log.Errorf("%s | gob Decode Err: %v, ServerId: %d", rpyPhase[OPB], err, int(coordinatorId))

			// interface{} にデコードして型を確認する
			var unknownMessage interface{}
			err = orderPhaseDialogInfo.dec.Decode(&unknownMessage)
			if err == nil {
				// 型を調べてログに出力
				log.Infof("Received a message of type: %v", reflect.TypeOf(unknownMessage))
			} else {
				// デコードに失敗した場合もログに出力
				log.Errorf("Failed to decode message for type detection: %v", err)
			}
			continue
		}

		go validatingOBEntry(&m, commitPhaseDialogInfo.enc)
	}

	// for {
	// 	// データをバッファリングするためのバッファ
	// 	var buf bytes.Buffer

	// 	// 一時的に 1024 バイトを読み込む（適宜サイズを調整）
	// 	_, err := io.CopyN(&buf, orderPhaseDialogInfo.conn, 1024)
	// 	if err != nil && err != io.EOF {
	// 		log.Errorf("Failed to read message data: %v", err)
	// 		continue
	// 	}

	// 	// バッファからgobデコードを試みる
	// 	dec := gob.NewDecoder(&buf)
	// 	var m ProposerOPBEntry
	// 	err = dec.Decode(&m)

	// 	if err == io.EOF {
	// 		nowTime := time.Now().UnixMilli()
	// 		log.Errorf("%s | coordinator closed connection | err: %v, time=%d", rpyPhase[OPB], err, nowTime)
	// 		break
	// 	}

	// 	// エラーが発生したら型を確認
	// 	if err != nil {
	// 		log.Errorf("%s | gob Decode Err: %v, ServerId: %d", rpyPhase[OPB], err, int(coordinatorId))

	// 		// バッファを再利用して型を確認する
	// 		var unknownMessage interface{}
	// 		dec = gob.NewDecoder(&buf) // 同じバッファを使ってもう一度デコード
	// 		err = dec.Decode(&unknownMessage)
	// 		if err == nil {
	// 			// 型を調べてログに出力
	// 			log.Infof("Received a message of type: %v", reflect.TypeOf(unknownMessage))
	// 		} else {
	// 			log.Errorf("Failed to decode message for type detection: %v", err)
	// 		}
	// 		continue
	// 	}

	// 	// 正常にデコードできた場合の処理
	// 	go validatingOBEntry(&m, commitPhaseDialogInfo.enc)
	// }
}

func receivingCADialMessages(coordinatorId ServerId) {
	dialogMgr.RLock()
	CADialogInfo := dialogMgr.conns[CPA][coordinatorId]
	dialogMgr.RUnlock()

	for {
		var m ProposerCPAEntry

		err := CADialogInfo.dec.Decode(&m)

		if err == io.EOF {
			log.Errorf("%v: Coordinator closed connection | err: %v", rpyPhase[CPA], err)
			break
		}

		if err != nil {
			log.Errorf("%v: Gob Decode Err: %v", rpyPhase[CPA], err)
			continue
		}

		go validatingCAEntry(&m, CADialogInfo.enc)

	}
}

func receivingCBDialMessages(coordinatorId ServerId) {
	dialogMgr.RLock()
	CBDialogInfo := dialogMgr.conns[CPB][coordinatorId]
	dialogMgr.RUnlock()

	for {
		var m ProposerCPBEntry

		err := CBDialogInfo.dec.Decode(&m)

		if err == io.EOF {
			log.Errorf("%v: Coordinator closed connection | err: %v", rpyPhase[CPB], err)
			break
		}

		if err != nil {
			log.Errorf("%v: Gob Decode Err: %v", rpyPhase[CPB], err)
			continue
		}

		go validatingCBEntry(&m, CBDialogInfo.enc)
	}
}

func receivingTIMEDialMessages(coordinatorId ServerId) {
	dialogMgr.RLock()
	TIMEDialogInfo := dialogMgr.conns[TIME][coordinatorId]
	dialogMgr.RUnlock()

	for {
		var m simulationStartTimeSyncMessage

		err := TIMEDialogInfo.dec.Decode(&m)

		if err == io.EOF {
			log.Errorf("%v: Coordinator closed connection | err: %v", rpyPhase[TIME], err)
			break
		}

		if err != nil {
			log.Errorf("%v: Gob Decode Err: %v", rpyPhase[TIME], err)
			continue
		}

		go syncSimulationStartTIme(&m)
	}
}

func syncSimulationStartTIme(m *simulationStartTimeSyncMessage) {
	simulationStartTime.Lock()
	if simulationStartTime.time < m.Time {
		simulationStartTime.time = m.Time
	}
	simulationStartTime.Unlock()
}
