package main

import (
	"encoding/gob"
	"errors"
	"io"
	"net"
	"time"
)

// validators' connections:
func runAsValidator() {
	defer proposerLookup.RUnlock()
	proposerLookup.RLock()

	for _, coordinatorId := range proposerLookup.m[OPA] {
		registerDialConn(coordinatorId, OPA, ListenerPortOPA)
		registerDialConn(coordinatorId, OPB, ListenerPortOPB)
		registerDialConn(coordinatorId, CPA, ListenerPortOCA)
		registerDialConn(coordinatorId, CPB, ListenerPortOCB)
	}

	log.Debugf("... registerDialConn completed ...")

	go receivingOADialMessages(proposerLookup.m[OPA][0])
	go receivingOBDialMessages(proposerLookup.m[OPB][0])
	go receivingCADialMessages(proposerLookup.m[CPA][0])
	go receivingCBDialMessages(proposerLookup.m[CPB][0])
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
		myDialAddr = myDialAdrIp + ":" + ServerList[ServerID].Ports[DialPortOPA+coordinatorIndex*8]
	case OPB:
		myDialAddr = myDialAdrIp + ":" + ServerList[ServerID].Ports[DialPortOPB+coordinatorIndex*8]
	case CPA:
		myDialAddr = myDialAdrIp + ":" + ServerList[ServerID].Ports[DialPortCPA+coordinatorIndex*8]
	case CPB:
		myDialAddr = myDialAdrIp + ":" + ServerList[ServerID].Ports[DialPortCPB+coordinatorIndex*8]
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

		if err != nil {
			log.Errorf("%s | gob Decode Err: %v", rpyPhase[OPB], err)
			continue
		}

		go validatingOBEntry(&m, commitPhaseDialogInfo.enc)
	}
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
