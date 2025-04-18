package main

import (
	"flag"
	"fmt"
	"strconv"
	"strings"
	"sync"
)

const NOP = 5 // Number of phases 4+1(TIME)

const MaxQueue = 10_000_000

const ArterySimulationDelay = 200.0
const InitialSyncBufferTime = 100.0

const (
	OPA = iota
	OPB
	CPA
	CPB
	TIME
)

const (
	ListenerPortOPA = iota
	ListenerPortOPB
	ListenerPortOCA
	ListenerPortOCB
	ListenerPortTIME
	DialPortOPA
	DialPortOPB
	DialPortCPA
	DialPortCPB
	DialPortTIME
)

// Below are three booth modes for testing the performance of VG in dynamic membership
// changes. I.e., booths may be different in ordering and consensus instances.
const (
	// BoothModeOCSB where Ordering and Consensus instances
	// take place in the Same Booth.
	BoothModeOCSB = iota
	// BoothModeOCDBWOP where Ordering and Consensus instances
	// take place in Different Booths With Overlapping Participants.
	BoothModeOCDBWOP
	// BoothModeOCDBNOP where Ordering and Consensus instances
	// take place in Different Booths with No Overlapping Participants.
	BoothModeOCDBNOP
)

var boothFromMode [3]int

var cmdPhase = []string{"OPA", "OPB", "CPA", "CPB", "TIME"}
var rpyPhase = []string{"R-OPA", "R-OPB", "R-CPA", "R-CPB", "TIME"}

type ServerId int
type Phase int
type BooIndices []int

func (b BooIndices) Contain(target int) bool {
	for _, v := range b {
		if v == target {
			return true
		}
	}
	return false
}

type ServerInfo struct {
	sync.RWMutex
	Index ServerId
	Ip    string
	Ports map[int]string
}

var ServerList []ServerInfo
var Quorum int

var serverIdLookup = struct {
	sync.RWMutex
	m map[string]ServerId
}{m: make(map[string]ServerId)}

var proposerLookup = struct {
	sync.RWMutex
	// m map[Phase]ServerId
	m map[Phase][]ServerId
}{m: make(map[Phase][]ServerId)}

// blockchainIdとproposerとなるserverIdとの対応関係
var blockchainInfo = struct {
	sync.RWMutex
	m map[int]map[Phase]ServerId
}{m: make(map[int]map[Phase]ServerId)}

var (
	BatchSize       int
	MsgSize         int
	MsgLoad         int64
	NumOfValidators int
	NumOfConn       int
	// BoothSize        int
	// Threshold        int
	LogLevel         int
	ServerID         int
	Delay            int
	GC               bool
	Role             int
	PerfMetres       bool
	PlainStorage     bool
	LatMetreInterval int64
	YieldCycle       int
	EasingDuration   int
	ConsWaitingSec   int
	ConsInterval     int
	ConfPath         string
	VehicleSpeed     int
	MainProposerId   int

	// for multiple proposer
	ProposerList  []ServerId
	ValidatorList struct {
		sync.Mutex
		list []ServerId
	}

	// whether consider communication possiblity between proposer and validator
	EvaluateComPossibilityFlag bool

	//If EvaluateComPossibilityFlag is true, it dicide whether to allow bypass route
	AllowBypassRoute bool

	// ArterySimulationDelay float64

	// BoothMode includes options for changing the booth dynamicity in evaluation
	BoothMode int

	BoothIDOfModeOCSB    int
	BoothIDOfModeOCDBWOP int
	BoothIDOfModeOCDBNOP int

	// Below parameters are used for catering factor evaluation.
	//SlowModeCycleNum    int
	//SleepTimeInSlowMode int
)

// 初期パラメータを設定
func loadCmdParameters() {
	flag.IntVar(&BatchSize, "b", 1, "batch size")
	flag.IntVar(&MsgSize, "m", 32, "message size")
	flag.Int64Var(&MsgLoad, "ml", 1000, "# of msg to be sent < "+strconv.Itoa(MaxQueue))
	flag.IntVar(&NumOfValidators, "w", 1, "number of worker threads")
	flag.IntVar(&NumOfConn, "c", 6, "max # of connections")
	// flag.IntVar(&BoothSize, "boo", 4, "# of vehicles in a booth")
	flag.IntVar(&ServerID, "id", 0, "serverID")
	flag.IntVar(&Delay, "d", 0, "network delay")
	flag.BoolVar(&PlainStorage, "s", false, "naive storage")
	flag.BoolVar(&GC, "gc", false, "garbage collection")
	flag.IntVar(&Role, "r", PROPOSER, "0 : Proposer | 1 : Validator")
	flag.BoolVar(&PerfMetres, "pm", true, "enabling performance metres")
	flag.Int64Var(&LatMetreInterval, "lm", 100, "latency measurement sample interval")
	flag.IntVar(&YieldCycle, "yc", 10, "yield sending after # cycles")
	flag.IntVar(&EasingDuration, "ed", 1, "each easing duration (ms)")
	flag.IntVar(&ConsWaitingSec, "cw", 10, "consensus waiting for new ordered blocks duration (seconds)")
	flag.IntVar(&ConsInterval, "ci", 500, "consensus instance interval (ms)")
	flag.IntVar(&LogLevel, "log", InfoLevel, "0: Panic | 1: Fatal | 2: Error | 3: Warn | 4: Info | 5: Debug")
	flag.StringVar(&ConfPath, "cfp", "./config/cluster_localhost.conf", "config file path")

	flag.IntVar(&VehicleSpeed, "vs", 80, "vehicle speed (km/h)")

	flag.IntVar(&BoothMode, "bm", 0, "booth mode: 0, 1, or 2")
	flag.IntVar(&BoothIDOfModeOCSB, "ocsb", 0, "BoothIDOfModeOCSB")
	// flag.IntVar(&BoothIDOfModeOCDBWOP, "ocdbwop", 1, "BoothIDOfModeOCDBWOP")
	// flag.IntVar(&BoothIDOfModeOCDBNOP, "ocdbnop", 5, "BoothIDOfModeOCDBNOP")
	flag.IntVar(&BoothIDOfModeOCDBWOP, "ocdbwop", 0, "BoothIDOfModeOCDBWOP")
	flag.IntVar(&BoothIDOfModeOCDBNOP, "ocdbnop", 0, "BoothIDOfModeOCDBNOP")

	//flag.IntVar(&SlowModeCycleNum, "sm", 3, "# of cycles going in slow mode")
	//flag.IntVar(&SleepTimeInSlowMode, "smt", 1, "slow mode cycle sleep time (second)")

	// for multiple proposer
	flag.BoolVar(&EvaluateComPossibilityFlag, "ecf", true, "whether consider communication possiblity between proposer and validator")
	// allow bypass route
	flag.BoolVar(&AllowBypassRoute, "abr", false, "If EvaluateComPossibilityFlag is true, it dicide whether to allow bypass route")

	// Add the proposer list flag
	var proposerIds string
	flag.StringVar(&proposerIds, "pl", "", "comma-separated list of proposer IDs")
	flag.IntVar(&MainProposerId, "mainp", 1, "main proposer id which make consensus")

	flag.Parse()

	// Split the proposerIds into an array and convert to int
	if proposerIds != "" {
		proposerStrIds := strings.Split(proposerIds, ",")
		for _, strId := range proposerStrIds {
			id, err := strconv.Atoi(strId)
			if err != nil {
				log.Fatalf("Invalid proposer ID: %s", strId)
			}
			ProposerList = append(ProposerList, ServerId(id))
		}
	} else {
		fmt.Println("No proposer IDs provided")
	}

	// Quorum = (BoothSize/3)*2 + 1
	// Threshold = Quorum - 1
}

const (
	PanicLevel = iota //0
	FatalLevel        //1
	ErrorLevel        //2
	WarnLevel         //3
	InfoLevel         //4
	DebugLevel        //5
	TraceLevel        //6
)
