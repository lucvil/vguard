#!/bin/bash
# test in localhost
./vginstance -id=$1 -r=$2 \
-b=100 \
-c=$3 \
-cfp="./config/cluster_localhost.conf" \
-ci=500 \
-cw=60 \
-d=$5 \
-ed=1 \
-gc=false \
-lm=100 \
-log=4 \
-m=$4 \
-ml=500000 \
-pm=true \
-s=true \
-w=1 \
-yc=0 \
-vs=$6

# To run in cluster, change -cfp to the cluster config file.
# E.g., -cfp="./config/cluster_4.conf"

# flag.IntVar(&BatchSize, "b", 1, "batch size")
# flag.IntVar(&MsgSize, "m", 32, "message size")
# flag.Int64Var(&MsgLoad, "ml", 1000, "# of msg to be sent < "+strconv.Itoa(MaxQueue))
# flag.IntVar(&NumOfValidators, "w", 1, "number of worker threads")
# flag.IntVar(&NumOfConn, "c", 6, "max # of connections")
# flag.IntVar(&BoothSize, "boo", 4, "# of vehicles in a booth")
# flag.IntVar(&ServerID, "id", 0, "serverID")
# flag.IntVar(&Delay, "d", 0, "network delay")
# flag.BoolVar(&PlainStorage, "s", false, "naive storage")
# flag.BoolVar(&GC, "gc", false, "garbage collection")
# flag.IntVar(&Role, "r", PROPOSER, "0 : Proposer | 1 : Validator")
# flag.BoolVar(&PerfMetres, "pm", true, "enabling performance metres")
# flag.Int64Var(&LatMetreInterval, "lm", 100, "latency measurement sample interval")
# flag.IntVar(&YieldCycle, "yc", 10, "yield sending after # cycles")
# flag.IntVar(&EasingDuration, "ed", 1, "each easing duration (ms)")
# flag.IntVar(&ConsWaitingSec, "cw", 10, "consensus waiting for new ordered blocks duration (seconds)")
# flag.IntVar(&ConsInterval, "ci", 500, "consensus instance interval (ms)")
# flag.IntVar(&LogLevel, "log", InfoLevel, "0: Panic | 1: Fatal | 2: Error | 3: Warn | 4: Info | 5: Debug")
# flag.StringVar(&ConfPath, "cfp", "./config/cluster_localhost.conf", "config file path")

# flag.IntVar(&BoothMode, "bm", 2, "booth mode: 0, 1, or 2")
# flag.IntVar(&BoothIDOfModeOCSB, "ocsb", 0, "BoothIDOfModeOCSB")
# flag.IntVar(&BoothIDOfModeOCDBWOP, "ocdbwop", 1, "BoothIDOfModeOCDBWOP")
# flag.IntVar(&BoothIDOfModeOCDBNOP, "ocdbnop", 5, "BoothIDOfModeOCDBNOP")