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
	arteryFilePath := "../artery/scenarios/vguard-test/results/speed" + strconv.Itoa(VehicleSpeed) + "/300vehicle/extended_time_id.json"
	// arteryFilePath := "./data.json"

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

// generateHash generates a SHA-256 hash of the given slice of integers
func generateBoothHash(indices []int) string {
	hasher := sha256.New()
	for _, v := range indices {
		hasher.Write([]byte(fmt.Sprintf("%d", v)))
	}
	return hex.EncodeToString(hasher.Sum(nil))
}

func generateBoothKey(booth Booth, numOfConn int) {
	cmdName := "./keyGen/generator"
	boothSize := len(booth.Indices)
	threshold := (boothSize / 3) * 2
	cmdArgs := []string{"-t=" + strconv.Itoa(threshold), "-n=" + strconv.Itoa(numOfConn), "-b=" + strconv.Itoa(booth.ID)}
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
		generateBoothKey(newBooth, NumOfConn)
	}

	return nowBoothId
}

func RoundToDecimal(value float64, places int) float64 {
	shift := math.Pow(10, float64(places))
	return math.Round(value*shift) / shift
}

func getBoothID() int {
	var pattern []int
	nowTime := time.Now().UnixMilli()
	pastTime := float64(nowTime) - float64(simulationStartTime)
	pastTime = pastTime/1000 + ArterySimulationDelay
	pastTime = RoundToDecimal(pastTime, 3)

	key := fmt.Sprintf("%.2f", pastTime)
	pattern = vehicleTimeData[key]
	pattern = append([]int{0}, pattern...)

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
