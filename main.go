package main

import (
	"fmt"
	"runtime"
	"sync"

	"github.com/sirupsen/logrus"
)

/*
Copyright (c) 2022

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

var log = logrus.New()
var metre latencyMetre
var vgInst sync.WaitGroup

func init() {
	//初期パラメータを設定
	loadCmdParameters()
	//ログファイルを設定
	setLogger()
	//config/cluster_localhost.confからサーバー情報をさがす
	parseConf(NumOfConn)
	// fetchKeys(Threshold, ServerID)
	initConns(NumOfConn)
	metre.init()

	fmt.Printf("-------------------------------\n")
	fmt.Printf("|- System  information board -|\n")
	fmt.Printf("|-----------------------------|\n")
	fmt.Printf("| Batch size\t| %3d\t|\n", BatchSize)
	fmt.Printf("| Message size\t| %3d\t|\n", MsgSize)
	fmt.Printf("| Server ID\t| %3d\t|\n", ServerID)
	fmt.Printf("| Log level\t| %3d\t|\n", LogLevel)
	fmt.Printf("| Init role\t| %3d\t|\n", Role)
	fmt.Printf("| # of servers\t| %3d\t|\n", NumOfConn)
	fmt.Printf("| Booth size\t| %3d\t|\n", BoothSize)
	fmt.Printf("| Quorum size\t| %3d\t|\n", Quorum)
	fmt.Printf("| Network delay\t| %3d\t|\n", Delay)
	fmt.Printf("-------------------------------\n")
	if PlainStorage {
		fmt.Printf("|-- Log shows at ./logs/s%d --|\n", ServerID)
		fmt.Printf("-------------------------------\n")
	}
}

func main() {
	//CPUの最大利用数の設定
	runtime.GOMAXPROCS(runtime.NumCPU())
	//待機グループのカウンターの増加
	vgInst.Add(1)

	log.Infof("V-Guard instance starts now")
	//非同期処理の開始(本体)
	go start()
	//vgInstのカウンターが0になるまでブロックします。
	vgInst.Wait()
}
