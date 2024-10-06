package main

import (
	"bufio"
	"errors"
	"os"
	"strconv"
	"strings"
)

func parseConf(numOfServers int) {
	var fileRows []string

	s, err := os.Open(ConfPath)
	if err != nil {
		panic(err)
	}

	scanner := bufio.NewScanner(s)
	scanner.Split(bufio.ScanLines)

	for scanner.Scan() {
		fileRows = append(fileRows, scanner.Text())
	}

	err = s.Close()
	if err != nil {
		log.Errorf("close fileServer failed | err: %v\n", err)
	}

	//first line is explanation
	// dont stop even if fileRow > numOfServers + 1
	if len(fileRows) < numOfServers*len(ProposerList)+1 {
		log.Errorf("Going to panic | fileRows: %v | n: %v", len(fileRows), numOfServers*len(ProposerList))
		panic(errors.New("number of servers in config file does not match with provided $n$"))
	}

	// for i := 0; i < len(fileRows); i++ {
	for i := 0; i < numOfServers+1; i++ {
		// Fist line is instructions
		if i == 0 {
			continue
		}

		var singleSL ServerInfo

		for j := 0; j < len(ProposerList); j++ {
			row := strings.Split(fileRows[(i-1)*len(ProposerList)+j+1], " ")

			if j == 0 {
				serverId, err := strconv.Atoi(row[0])
				if err != nil {
					panic(err)
				}

				singleSL.Index = ServerId(serverId)

				singleSL.Ip = row[1]

				singleSL.Ports = make(map[int]string)
			}

			singleSL.Ports[ListenerPortOPA+j*8] = row[3]
			singleSL.Ports[ListenerPortOPB+j*8] = row[4]
			singleSL.Ports[ListenerPortOCA+j*8] = row[5]
			singleSL.Ports[ListenerPortOCB+j*8] = row[6]
			singleSL.Ports[DialPortOPA+j*8] = row[7]
			singleSL.Ports[DialPortOPB+j*8] = row[8]
			singleSL.Ports[DialPortCPA+j*8] = row[9]
			singleSL.Ports[DialPortCPB+j*8] = row[10]

			serverIdLookup.Lock()
			serverIdLookup.m[singleSL.Ip+":"+row[7]] = singleSL.Index
			serverIdLookup.m[singleSL.Ip+":"+row[8]] = singleSL.Index
			serverIdLookup.m[singleSL.Ip+":"+row[9]] = singleSL.Index
			serverIdLookup.m[singleSL.Ip+":"+row[10]] = singleSL.Index
			serverIdLookup.Unlock()

			if j == len(ProposerList)-1 {
				ServerList = append(ServerList, singleSL)
				log.Debugf("Config file fetched | S%d -> %v:%v \n", singleSL.Index, singleSL.Ip, singleSL.Ports)
			}
		}
	}
}
