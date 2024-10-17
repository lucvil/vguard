package main

import (
	"io"
	"net"
	"slices"
	"sync"
	"sync/atomic"
	"time"
)

func runAsProposer(proposerId ServerId) {

	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recovered from panic in runAsProposer: %v\n", r)
		}
	}()

	var wg sync.WaitGroup
	// NOP = 4 (Number of phases)
	wg.Add(NOP)

	fetchArteryData()

	for i := 0; i < NOP; i++ {
		//validator接続の受け入れ
		blockchainInfo.Lock()
		if blockchainInfo.m[ServerID] == nil {
			blockchainInfo.m[ServerID] = make(map[Phase]ServerId)
		}
		blockchainInfo.m[ServerID][Phase(i)] = ServerId(ServerID)
		blockchainInfo.Unlock()
		go acceptValidatorConns(proposerId, &wg, i)
	}

	wg.Wait()

	wg.Add(NOP + 1)

	for i := 0; i < NOP; i++ {
		//validator接続の受け入れ
		go acceptProposerConns(proposerId, &wg, i)
	}

	time.Sleep(10 * time.Second) // 10秒待機

	proposerLookup.RLock()
	defer proposerLookup.RUnlock()

	for _, coordinatorId := range proposerLookup.m[OPA] {
		if coordinatorId == ServerId(ServerID) {
			continue
		}

		registerDialConn(coordinatorId, OPA, ListenerPortOPA)
		registerDialConn(coordinatorId, OPB, ListenerPortOPB)
		registerDialConn(coordinatorId, CPA, ListenerPortOCA)
		registerDialConn(coordinatorId, CPB, ListenerPortOCB)
	}

	for _, coordinatorId := range proposerLookup.m[OPA] {
		if coordinatorId == ServerId(ServerID) {
			continue
		}

		go relayBetweenProposerMessage(proposerLookup.m[OPA][coordinatorId], OPA)
		go relayBetweenProposerMessage(proposerLookup.m[OPB][coordinatorId], OPB)
		go relayBetweenProposerMessage(proposerLookup.m[CPA][coordinatorId], CPA)
		go relayBetweenProposerMessage(proposerLookup.m[CPB][coordinatorId], CPB)
	}

	//すべてのバリデーターが揃うまで待機
	wg.Wait()
	log.Infof("Network connections are now set | # of phases: %v", NOP)

	// if ServerID != 1 {
	// 	//以降の処理を停止
	// 	fmt.Printf("aaaa")
	// 	time.Sleep(1000)
	// 	return
	// }

	//boothを作成、ID0とID1はかならず含む
	//NumOfConn  "c", 6, "max # of connections"
	// prepareBooths(NumOfConn, BoothSize)

	fetchProToValComTimeMap([]ServerId{ServerId(ServerID)})
	ValidatorList.Lock()
	fetchValToProComTimeMap(ValidatorList.list)
	ValidatorList.Unlock()

	//データを事前に用意、requestQueueに格納
	txGenerator(MsgSize)

	//NumOfValidators, "w", 1, "number of worker threads"
	simulationStartTime = time.Now().UnixMilli()

	for i := 0; i < NumOfValidators; i++ {
		go startOrderingPhaseA(i)
	}

	go startConsensusPhaseA()
}

func closeTCPListener(l *net.TCPListener, phaseNum int) {
	err := (*l).Close()
	if err != nil {
		log.Errorf("close Phase %v TCP listener failed | err: %v", phaseNum, err)
	}
}

// 接続の受け入れ
func acceptValidatorConns(leaderId ServerId, wg *sync.WaitGroup, phase int) {
	addr, err := net.ResolveTCPAddr("tcp4", ServerList[leaderId].Ip+":"+ServerList[leaderId].Ports[phase])
	if err != nil {
		log.Error(err)
		return
	}
	listener, err := net.ListenTCP("tcp4", addr)

	if err != nil {
		log.Error(err)
		return
	}
	defer closeTCPListener(listener, phase)
	log.Infof("%s | listener is up at %s", cmdPhase[phase], listener.Addr().String())

	connected := 0

	//接続を無限ループ内で受付(validator向け)
	for {
		conn, err := listener.AcceptTCP()
		if err != nil {
			log.Errorf("%s listener err: %v", cmdPhase[phase], err)
			return
		}

		sid, err := connRegistration(*conn, phase)
		if err != nil {
			log.Errorf("server registration err: %v", err)
			return
		} else {
			ValidatorList.Lock()
			if !(slices.Contains(ValidatorList.list, sid)) {
				ValidatorList.list = append(ValidatorList.list, sid)
			}
			ValidatorList.Unlock()
		}

		switch phase {
		case OPA:
			go handleOPAConns(conn, sid)
		case OPB:
			go handleOPBConns(conn, sid)
		case CPA:
			go handleCPAConns(conn, sid)
		case CPB:
			go handleCPBConns(conn, sid)
		}

		connected++
		if connected == NumOfConn-len(ProposerList) {
			log.Debugf("%s listener | all %d expected servers connected", cmdPhase[phase], connected)
			wg.Done()
			break
		}
	}
}

func handleOPAConns(sConn *net.TCPConn, sid ServerId) {
	//
	//
}

// バリデータからのTCP応答を処理する
func handleOPBConns(sConn *net.TCPConn, sid ServerId) {
	for {
		var m ValidatorOPAReply

		if err := concierge.n[OPB][sid].dec.Decode(&m); err == nil {
			//mをlog.infofで出力する
			// log.Infof("receive OB Reply message")
			if blockchainInfo.m[m.BlockchainId][OPB] != ServerId(ServerID) {
				sendMessage := BetweenProposerMsg{
					Message:   m,
					Sender:    int(sid),
					Recipient: int(blockchainInfo.m[m.BlockchainId][OPB]),
					Phase:     OPB,
				}
				dialogMgr.RLock()
				sendDialogInfo := dialogMgr.conns[OPB][blockchainInfo.m[m.BlockchainId][OPB]]
				dialSendBack(sendMessage, sendDialogInfo.enc, OPB)
				continue
			}

			go asyncHandleOBReply(&m, sid)
		} else if err == io.EOF {
			log.Errorf("%s | server %v closed connection | err: %v", cmdPhase[OPB], sid, err)
			break
		} else {
			log.Errorf("%s | gob decode Err: %v | conn with ser: %v | remoteAddr: %v",
				cmdPhase[OPB], err, sid, (*sConn).RemoteAddr())
			continue
		}
	}
}

func handleCPAConns(sConn *net.TCPConn, sid ServerId) {

	receiveCounter := int64(0)

	for {
		var m ValidatorCPAReply

		err := concierge.n[CPA][sid].dec.Decode(&m)

		counter := atomic.AddInt64(&receiveCounter, 1)

		if err == io.EOF {
			log.Errorf("%v | server %v closed connection | err: %v", time.Now(), sid, err)
			break
		}

		if err != nil {
			log.Errorf("Gob Decode Err: %v | conn with ser: %v | remoteAddr: %v | Now # %v", err, sid, (*sConn).RemoteAddr(), counter)
			continue
		}

		if &m != nil {
			if blockchainInfo.m[m.BlockchainId][CPA] != ServerId(ServerID) {
				sendMessage := BetweenProposerMsg{
					Message:   m,
					Sender:    int(sid),
					Recipient: int(blockchainInfo.m[m.BlockchainId][CPA]),
					Phase:     CPA,
				}
				dialogMgr.RLock()
				sendDialogInfo := dialogMgr.conns[CPA][blockchainInfo.m[m.BlockchainId][CPA]]
				dialSendBack(sendMessage, sendDialogInfo.enc, CPA)
				continue
			}

			go asyncHandleCPAReply(&m, sid)
		} else {
			log.Errorf("received message is nil")
		}
	}
}

func handleCPBConns(sConn *net.TCPConn, sid ServerId) {
	//
	//
}

// 接続の受け入れ
func acceptProposerConns(leaderId ServerId, wg *sync.WaitGroup, phase int) {
	addr, err := net.ResolveTCPAddr("tcp4", ServerList[leaderId].Ip+":"+ServerList[leaderId].Ports[phase])
	if err != nil {
		log.Error(err)
		return
	}
	listener, err := net.ListenTCP("tcp4", addr)

	if err != nil {
		log.Error(err)
		return
	}
	defer closeTCPListener(listener, phase)
	log.Infof("%s | listener is up at %s", cmdPhase[phase], listener.Addr().String())

	connected := 0
	if connected == len(ProposerList)-1 {
		wg.Done()
	}

	//接続を無限ループ内で受付(validator向け)
	for {
		conn, err := listener.AcceptTCP()
		if err != nil {
			log.Errorf("%s listener err: %v", cmdPhase[phase], err)
			return
		}

		sid, err := connRegistration(*conn, phase)
		if err != nil {
			log.Errorf("server registration err: %v", err)
			return
		}

		switch phase {
		case OPA:
			go handleProposerOPAConns(conn, sid)
		case OPB:
			go handleProposerOPBConns(conn, sid)
		case CPA:
			go handleProposerCPAConns(conn, sid)
		case CPB:
			go handleProposerCPBConns(conn, sid)
		}

		connected++
		if connected == len(ProposerList)-1 {
			log.Debugf("%s listener | all %d expected servers connected", cmdPhase[phase], connected)
			wg.Done()
			// break
		}
	}
}

func handleProposerOPAConns(sConn *net.TCPConn, sid ServerId) {
	//
	//
}

// バリデータからのTCP応答を処理する
func handleProposerOPBConns(sConn *net.TCPConn, sid ServerId) {
	for {
		var receivedMessage BetweenProposerMsg

		if err := concierge.n[OPB][sid].dec.Decode(&receivedMessage); err == nil {
			//mをlog.infofで出力する
			// log.Infof("receive OB Reply message")
			m, ok := receivedMessage.Message.(ValidatorOPAReply)
			if !ok {
				log.Errorf("Failed to cast message to ValidatorOPAReply | received type: %T", receivedMessage.Message)
				continue
			}

			go asyncHandleOBReply(&m, ServerId(receivedMessage.Sender))
		} else if err == io.EOF {
			log.Errorf("%s | server %v closed connection | err: %v", cmdPhase[OPB], sid, err)
			break
		} else {
			log.Errorf("%s | gob decode Err: %v | conn with ser: %v | remoteAddr: %v",
				cmdPhase[OPB], err, sid, (*sConn).RemoteAddr())
			continue
		}
	}
}

func handleProposerCPAConns(sConn *net.TCPConn, sid ServerId) {

	receiveCounter := int64(0)

	for {
		var receivedMessage BetweenProposerMsg

		err := concierge.n[CPA][sid].dec.Decode(&receivedMessage)

		counter := atomic.AddInt64(&receiveCounter, 1)

		if err == io.EOF {
			log.Errorf("%v | server %v closed connection | err: %v", time.Now(), sid, err)
			break
		}

		if err != nil {
			log.Errorf("Gob Decode Err: %v | conn with ser: %v | remoteAddr: %v | Now # %v", err, sid, (*sConn).RemoteAddr(), counter)
			continue
		}

		if &receivedMessage != nil {
			m, ok := receivedMessage.Message.(ValidatorCPAReply)
			if !ok {
				log.Errorf("Failed to cast message to ValidatorCPAReply | received type: %T", receivedMessage.Message)
				continue
			}
			go asyncHandleCPAReply(&m, ServerId(receivedMessage.Sender))
		} else {
			log.Errorf("received message is nil")
		}
	}
}

func handleProposerCPBConns(sConn *net.TCPConn, sid ServerId) {
	//
	//
}

func relayBetweenProposerMessage(coordinatorId ServerId, phase int) {
	dialogMgr.RLock()
	postPhaseDialogInfo := dialogMgr.conns[phase][coordinatorId]
	dialogMgr.RUnlock()

	for {
		var receivedMessage BetweenProposerMsg

		receiveMsgErr := postPhaseDialogInfo.dec.Decode(&receivedMessage)

		if receiveMsgErr == io.EOF {
			nowTime := time.Now().UnixMilli()
			log.Errorf("%v | coordinator closed connection | err: %v, time=%d", rpyPhase[phase], receiveMsgErr, nowTime)
			log.Warnf("Lost connection with the proposer (S%v); quitting program", postPhaseDialogInfo.SID)
			vgInst.Done()
			break
		}

		if receiveMsgErr != nil {
			log.Errorf("Gob Decode Err: %v", receiveMsgErr)
			continue
		}

		var recipient = receivedMessage.Recipient
		needDetour, detourNextNode := checkComPathToValidator(recipient)
		var nextNode int
		var sendMessage any

		if needDetour {
			nextNode = detourNextNode
			sendMessage = receivedMessage
		} else {
			nextNode = recipient
			sendMessage = receivedMessage.Message
		}

		if nextNode == -1 {
			log.Infof("server %v cannot communicate with any proposer", recipient)
			continue
		}

		if concierge.n[phase][nextNode] == nil {
			log.Errorf("server %v is not registered in phase %v | msg tried to sent %v:", nextNode, phase, sendMessage)
			continue
		}

		sendMsgErr := concierge.n[phase][nextNode].enc.Encode(sendMessage)
		if sendMsgErr != nil {
			broadcastError = true
			switch sendMsgErr {
			case io.EOF:
				log.Errorf("server %v closed connection |needDetour: %t| err: %v", concierge.n[phase][nextNode].SID, needDetour, sendMsgErr)
				break
			default:
				log.Errorf("sent to server %v failed |needDetour: %t| err: %v", concierge.n[phase][nextNode].SID, needDetour, sendMsgErr)
			}
		}
	}
}
