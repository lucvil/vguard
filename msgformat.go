package main

// This file describes the message format in VGuard. Each message is encoded and
// decoded by gob encoders. An encoded message will be transmitted and decoded to
// another server. Thus, all feilds must be capitilized.
//

type Proposal struct {
	Timestamp   int64
	Transaction []byte
}

type Entry struct {
	TimeStamp int64
	Tx        []byte
}

type simulationStartTimeSyncMessage struct {
	Time       int64
	ProposerId int
}

type ProposerOPAEntry struct {
	Booth
	BlockId      int64
	BlockchainId int
	Entries      map[int]Entry
	Hash         []byte
}

type ValidatorOPAReply struct {
	BlockchainId int
	BlockId      int64
	ParSig       []byte
}

type ProposerOPBEntry struct {
	Booth
	BlockchainId int
	BlockId      int64
	CombSig      []byte
	//Entries only enabled calling from ProposerCPAEntry
	Entries map[int]Entry
	Hash    []byte
}

type ProposerCPAEntry struct {
	//PrevOPBEntries is only piggybacked when the participant did not see the ordered entries
	PrevOPBEntries []ProposerOPBEntry

	Booth
	BlockchainId int
	BIDs         []int64          // starting BlockID in this range (included in tx)
	ConsInstID   int              // ending BlockID in this range (included in tx)
	RangeHash    map[int64][]byte // <BlockID, hash>
	TotalHash    []byte
}

func (p *ProposerCPAEntry) SetPrevOPBEntries(m []ProposerOPBEntry) {
	p.PrevOPBEntries = m
}

type ValidatorCPAReply struct {
	BlockchainId int
	ConsInstID   int
	ParSig       []byte
}

type ProposerCPBEntry struct {
	Booth
	BlockchainId int
	ConsInstID   int
	ComSig       []byte
	Hash         []byte
}

type ValidatorCPBReply struct {
	BlockchainId int
	ConsInstID   int
	Done         bool
	//ParSig	[]byte
}

type BetweenProposerMsg struct {
	Message   any
	Sender    int
	Recipient int
	Phase     int
}
