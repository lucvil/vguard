package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"gonum.org/v1/gonum/stat/combin"
)

type Booth struct {
	ID      int
	Indices []int

	//key is pub key hash; value is server id
	//Identities map[[32]byte]int
}

var proToValComTimeMap = struct {
	sync.RWMutex
	timeMap map[ServerId]map[string][]ServerId
}{timeMap: make(map[ServerId]map[string][]ServerId)}

var valToProComTimeMap = struct {
	sync.RWMutex
	timeMap map[ServerId]map[string][]ServerId
}{timeMap: make(map[ServerId]map[string][]ServerId)}

func (b *Booth) String() (string, error) {
	if len(b.Indices) == 0 {
		return "", errors.New("booth is nil")
	}

	out := make([]string, len(b.Indices))
	for i, v := range b.Indices {
		out[i] = strconv.Itoa(v)
	}

	return strings.Join(out, ""), nil
}

// // booMgr is the queue of all enqueued booths
// var booMgr = struct {
// 	sync.RWMutex
// 	b []Booth
// }{}

// booMgr is the queue of all enqueued booths
var booMgr = struct {
	sync.RWMutex
	b     []Booth
	index map[string]int
}{index: make(map[string]int)}

func fetchArteryData() {
	// arteryFilePath := "../artery/scenarios/vguard-test/results/speed" + strconv.Itoa(VehicleSpeed) + "/300vehicle/extended_time_id.json"
	arteryFilePath := "./artery-data/data.json"

	// JSONファイルを読み込む
	file, err := os.Open(arteryFilePath)
	if err != nil {
		fmt.Printf("Error opening file: %s\n", err)
		return
	}
	defer file.Close()

	// ファイルの内容を読み込む
	byteValue, err := io.ReadAll(file)
	if err != nil {
		fmt.Printf("Error reading file: %s\n", err)
		return
	}

	// JSONデコード
	if err := json.Unmarshal(byteValue, &vehicleTimeData); err != nil {
		fmt.Printf("Error unmarshalling JSON: %s\n", err)
		return
	}

}

func getThreshold(boothSize int) int {
	return (boothSize / 3) * 2
}

func fetchProToValComTimeMap(proposerList []ServerId) {
	for _, proposerId := range proposerList {
		filePath := "./artery-data/communication-data-" + strconv.Itoa(int(proposerId)) + ".json"

		// JSONファイルを読み込む
		file, err := os.Open(filePath)
		if err != nil {
			fmt.Printf("Error opening file: %s\n", err)
			return
		}
		defer file.Close()

		// ファイルの内容を読み込む
		byteValue, err := io.ReadAll(file)
		if err != nil {
			fmt.Printf("Error reading file: %s\n", err)
			return
		}

		// JSONデコード
		var proToValTimeMapItem map[string][]ServerId

		if err := json.Unmarshal(byteValue, &proToValTimeMapItem); err != nil {
			fmt.Printf("Error unmarshalling JSON: %s\n", err)
			return
		}

		proToValComTimeMap.Lock()
		if _, ok := proToValComTimeMap.timeMap[proposerId]; !ok {
			proToValComTimeMap.timeMap[proposerId] = make(map[string][]ServerId)
		}
		proToValComTimeMap.timeMap[proposerId] = proToValTimeMapItem
		proToValComTimeMap.Unlock()
	}
}

func fetchValToProComTimeMap(validatorList []ServerId) {
	for _, validatorId := range validatorList {
		filePath := "./artery-data/communication-data-" + strconv.Itoa(int(validatorId)) + ".json"

		// JSONファイルを読み込む
		file, err := os.Open(filePath)
		if err != nil {
			fmt.Printf("Error opening file: %s\n", err)
			return
		}
		defer file.Close()

		// ファイルの内容を読み込む
		byteValue, err := io.ReadAll(file)
		if err != nil {
			fmt.Printf("Error reading file: %s\n", err)
			return
		}

		// JSONデコード
		var valToProTimeMapItem map[string][]ServerId

		if err := json.Unmarshal(byteValue, &valToProTimeMapItem); err != nil {
			fmt.Printf("Error unmarshalling JSON: %s\n", err)
			return
		}

		valToProComTimeMap.Lock()
		if _, ok := valToProComTimeMap.timeMap[validatorId]; !ok {
			valToProComTimeMap.timeMap[validatorId] = make(map[string][]ServerId)
		}
		valToProComTimeMap.timeMap[validatorId] = valToProTimeMapItem
		valToProComTimeMap.Unlock()
	}
}

// generateHash generates a SHA-256 hash of the given slice of integers
func generateBoothHash(indices []int) string {
	hasher := sha256.New()
	for _, v := range indices {
		hasher.Write([]byte(fmt.Sprintf("%d", v)))
	}
	return hex.EncodeToString(hasher.Sum(nil))
}

func generateBoothKey(booth Booth, numOfConn int, blockchainId int) {
	cmdName := "./keyGen/generator"
	boothSize := len(booth.Indices)
	threshold := (boothSize / 3) * 2
	fmt.Printf("make this booth id key: %d, %v\n", booth.ID, booth.Indices)
	cmdArgs := []string{"-t=" + strconv.Itoa(threshold), "-n=" + strconv.Itoa(numOfConn), "-b=" + strconv.Itoa(booth.ID), "-p=" + strconv.Itoa(blockchainId)}
	cmd := exec.Command(cmdName, cmdArgs...)
	// コマンドの標準出力を取得
	_, err := cmd.Output()
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		return
	}

}

// checkExactMatchInBooMgr checks if any Booth's Indices exactly match the pattern using the index
func checkExactMatchInBooMgr(pattern []int) int {
	sort.Ints(pattern)
	key := generateBoothHash(pattern)

	// Use a single write lock to ensure atomicity
	booMgr.Lock()
	defer booMgr.Unlock()

	nowBoothId, exists := booMgr.index[key]

	if !exists {
		// Add the new booth to the booMgr
		newBooth := Booth{
			ID:      len(booMgr.b),
			Indices: pattern,
		}
		booMgr.b = append(booMgr.b, newBooth)
		booMgr.index[generateBoothHash(pattern)] = newBooth.ID
		nowBoothId = newBooth.ID

		// Generate the key for the new booth
		var blockchainId = ServerID
		generateBoothKey(newBooth, NumOfConn, blockchainId)
	}

	return nowBoothId
}

func RoundToDecimal(value float64, places int) float64 {
	shift := math.Pow(10, float64(places))
	return math.Round(value*shift) / shift
}

func getNowTimeKey() string {
	nowTime := time.Now().UnixMilli()
	pastTime := float64(nowTime) - float64(simulationStartTime)
	pastTime = pastTime/1000 + ArterySimulationDelay
	pastTime = RoundToDecimal(pastTime, 3)

	key := fmt.Sprintf("%.2f", pastTime)

	return key
}

func getBoothID() int {
	var pattern []int
	nowTime := time.Now().UnixMilli()
	pastTime := float64(nowTime) - float64(simulationStartTime)
	pastTime = pastTime/1000 + ArterySimulationDelay
	pastTime = RoundToDecimal(pastTime, 3)

	key := getNowTimeKey()
	pattern = vehicleTimeData[key][ServerId(ServerID)]
	//logにpatternとpastTimeを記述
	// log.Infof("pattern:%v, pastTime: %f", pattern, key)

	pattern = append([]int{ServerID}, pattern...)

	nowBoothId := checkExactMatchInBooMgr(pattern)
	return nowBoothId
}

// booMgrnにIDが0と1のものがかならず入るようにしてboothを作成
func prepareBooths(numOfConns, boothsize int) {
	total, boothsize := numOfConns-1, boothsize-1

	if total < boothsize {
		total = boothsize
		log.Warnf("total is %v, which is less than %v; setting total=boothsize", total, boothsize)
	}

	boothsIndices := combin.Combinations(total, boothsize)
	log.Infof("total: %v, boothsize: %v| generated booth indices: %v", total, boothsize, boothsIndices)

	for boothId, memberIds := range boothsIndices {
		log.Debugf("BoothID: %v -> Members: %v", boothId, memberIds)

		// Filtering out booths without memberID 0, as a booth must contain proposer and pivot validator.
		// MemberID 0 is a symbol for the combination of the proposer and pivot validator. Thus, the number of members
		// now is one less than the actual number of members. The next increments member IDs and member 0 back to the
		// booth.

		pivotFlag := false
		for i := 0; i < len(memberIds); i++ {
			if memberIds[i] == 0 {
				pivotFlag = true
			}
			memberIds[i]++
		}

		if !pivotFlag {
			continue
		}

		memberIds = append(memberIds, 0)

		booMgr.b = append(booMgr.b, Booth{
			ID:      boothId,
			Indices: memberIds,
		})

	}
	log.Infof("enqueued booths: %v", booMgr.b)
}

var broadcastError = false

// broadcastToBooth is used by the proposer to broadcast a given message to all members in a given booth
func broadcastToBooth(e interface{}, phase int, boothID int) {
	if broadcastError {
		return
	}

	boo := booMgr.b[boothID]

	for _, i := range boo.Indices {
		if ServerID == i {
			continue
		}

		if concierge.n[phase][i] == nil {
			log.Errorf("server %v is not registered in phase %v | msg tried to sent %v:", i, phase, e)
			continue
		}

		err := concierge.n[phase][i].enc.Encode(e)
		if err != nil {
			broadcastError = true
			switch err {
			case io.EOF:
				log.Errorf("server %v closed connection | err: %v", concierge.n[phase][i].SID, err)
				break
			default:
				log.Errorf("sent to server %v failed | err: %v", concierge.n[phase][i].SID, err)
			}
		}
	}

	nowTime := time.Now().UnixMilli()
	switch e.(type) {
	case ProposerOPAEntry:
		log.Infof("broadcasted to booth %v , brock: %d| time: %v", boothID, e.(ProposerOPAEntry).BlockId, nowTime)
	}
}

func broadcastToNewBooth(regularMsg interface{}, phase int, boothID int, newMemberIDs []int, newMsg interface{}) {
	if broadcastError {
		return
	}

	boo := booMgr.b[boothID]

	for _, i := range boo.Indices {
		var err error

		if ServerID == i {
			continue
		}

		if concierge.n[phase][i] == nil {
			log.Errorf("server %v is not registered in phase %v | msg tried to sent %v:", i, phase, regularMsg)
			continue
		}

		newMemberFlag := false
		for _, newMember := range newMemberIDs {
			if newMember == i {
				//log.Errorf("newMember: %v is not in Booth: %v", newMember, boo.Indices)
				newMemberFlag = true
				err = concierge.n[phase][i].enc.Encode(newMsg)
			}
		}

		if newMemberFlag {
			continue
		}

		err = concierge.n[phase][i].enc.Encode(regularMsg)
		if err != nil {
			broadcastError = true
			switch err {
			case io.EOF:
				log.Errorf("server %v closed connection | err: %v", concierge.n[phase][i].SID, err)
				break
			default:
				log.Errorf("sent to server %v failed | err: %v", concierge.n[phase][i].SID, err)
			}
		}
	}
}

func checkComPathToValidator(validatorId int) (bool, int) {
	var needDetour bool
	var detourNextNode int
	timeKey := getNowTimeKey()

	communicableValidatorList := proToValComTimeMap.timeMap[ServerId(ServerID)][timeKey]
	if slices.Contains(communicableValidatorList, ServerId(validatorId)) {
		needDetour = false
		detourNextNode = validatorId
		return needDetour, detourNextNode
	} else {
		communicableProposerList := valToProComTimeMap.timeMap[ServerId(validatorId)][timeKey]
		needDetour = true
		if len(communicableProposerList) == 0 {
			detourNextNode = -1
		} else {
			detourNextNode = int(communicableProposerList[0])
		}
		return needDetour, detourNextNode
	}
}

// broadcastToBooth is used by the proposer to broadcast a given message to all members in a given booth
func broadcastToBoothWithComCheck(e interface{}, phase int, boothID int) {
	if broadcastError {
		return
	}

	boo := booMgr.b[boothID]

	for _, i := range boo.Indices {
		if ServerID == i {
			continue
		}

		needDetour, detourNextNode := checkComPathToValidator(i)
		var nextNode int
		var message any
		if needDetour {
			nextNode = detourNextNode
			message = BetweenProposerMsg{
				message:   e,
				sender:    ServerID,
				recipient: i,
				phase:     phase,
			}
		} else {
			nextNode = i
			message = e
		}

		if concierge.n[phase][nextNode] == nil {
			log.Errorf("server %v is not registered in phase %v | msg tried to sent %v:", i, phase, e)
			continue
		}

		err := concierge.n[phase][nextNode].enc.Encode(message)
		if err != nil {
			broadcastError = true
			switch err {
			case io.EOF:
				log.Errorf("server %v closed connection |needDetour: %t| err: %v", concierge.n[phase][nextNode].SID, needDetour, err)
				break
			default:
				log.Errorf("sent to server %v failed |needDetour: %t| err: %v", concierge.n[phase][nextNode].SID, needDetour, err)
			}
		}
	}

	nowTime := time.Now().UnixMilli()
	switch e.(type) {
	case ProposerOPAEntry:
		log.Infof("broadcasted to booth %v , brock: %d| time: %v", boothID, e.(ProposerOPAEntry).BlockId, nowTime)
	}
}

func broadcastToNewBoothWithComCheck(regularMsg interface{}, phase int, boothID int, newMemberIDs []int, newMsg interface{}) {
	if broadcastError {
		return
	}

	boo := booMgr.b[boothID]

	for _, i := range boo.Indices {
		var err error

		if ServerID == i {
			continue
		}

		needDetour, detourNextNode := checkComPathToValidator(i)
		var nextNode int
		var message any
		if needDetour {
			nextNode = detourNextNode
		} else {
			nextNode = i
		}

		if concierge.n[phase][nextNode] == nil {
			log.Errorf("server %v is not registered in phase %v | msg tried to sent %v:", i, phase, regularMsg)
			continue
		}

		newMemberFlag := false
		for _, newMember := range newMemberIDs {
			if newMember == i {
				//log.Errorf("newMember: %v is not in Booth: %v", newMember, boo.Indices)
				newMemberFlag = true
				if needDetour {
					message = BetweenProposerMsg{
						message:   newMsg,
						sender:    ServerID,
						recipient: i,
						phase:     phase,
					}
				} else {
					message = newMsg
				}
				err = concierge.n[phase][nextNode].enc.Encode(message)
			}
		}

		if newMemberFlag {
			continue
		}

		if needDetour {
			message = BetweenProposerMsg{
				message:   regularMsg,
				sender:    ServerID,
				recipient: i,
				phase:     phase,
			}
		} else {
			message = regularMsg
		}

		err = concierge.n[phase][i].enc.Encode(message)
		if err != nil {
			broadcastError = true
			switch err {
			case io.EOF:
				log.Errorf("server %v closed connection | err: %v", concierge.n[phase][i].SID, err)
				break
			default:
				log.Errorf("sent to server %v failed | err: %v", concierge.n[phase][i].SID, err)
			}
		}
	}
}

// broadcastToAll is used by the proposer to broadcast a given message to all connected members.
func broadcastToAll(e interface{}, phase int) {
	if broadcastError {
		return
	}

	for i := 0; i < len(concierge.n[phase]); i++ {
		if ServerID == i {
			continue
		}

		if concierge.n[phase][i] == nil {
			log.Errorf("server %v is not registered in phase %v | msg tried to sent %v:", i, phase, e)
			continue
		}

		err := concierge.n[phase][i].enc.Encode(e)
		if err != nil {
			broadcastError = true
			switch err {
			case io.EOF:
				log.Errorf("server %v closed connection | err: %v", concierge.n[phase][i].SID, err)
				break
			default:
				log.Errorf("sent to server %v failed | err: %v", concierge.n[phase][i].SID, err)
			}
		}
	}
}
