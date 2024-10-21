#!/bin/bash

# 子プロセスが終了する際に全ての子プロセスも終了するようにする
trap "kill 0" EXIT

# スタックサイズの制限を解除
ulimit -s unlimited

rm -rf keys

rm -rf logs
mkdir logs

./scripts/build.sh

PROPOSER_NUM=2
VALIDATOR_NUM=10
PROPOSER_NUM=3
VALIDATOR_NUM=250
PARTICIPANT_NUM=$((PROPOSER_NUM + VALIDATOR_NUM))
# プロポーザリスト (0,1,2)
PROPOSER_LIST=$(seq -s, 0 $((PROPOSER_NUM - 1)))
MESSAGE_SIZE=32
NETWORK_DELAY=0
VEHICLE_SPEED=80
BYPASSFLAG=true

# バリデータをバックグラウンドで起動し、プロセスIDを格納
for i in $(seq 0 $((PROPOSER_NUM - 1))); do
  ./scripts/bypass_route_run.sh $i 0 $PARTICIPANT_NUM $MESSAGE_SIZE $NETWORK_DELAY $VEHICLE_SPEED $PROPOSER_LIST $BYPASSFLAG&
  sleep 0.5
done

# バリデータをバックグラウンドで起動し、プロセスIDを格納
for i in $(seq $PROPOSER_NUM $((PARTICIPANT_NUM - 1))); do
  ./scripts/bypass_route_run.sh $i 1 $PARTICIPANT_NUM $MESSAGE_SIZE $NETWORK_DELAY $VEHICLE_SPEED $PROPOSER_LIST $BYPASSFLAG& pid=$!
  declare "pid$i=$pid"  # プロセスIDを動的変数名で保存
  sleep 0.1
done

# 各バリデータの終了を待機
for i in $(seq $PROPOSER_NUM $((PARTICIPANT_NUM - 1))); do
  eval "wait \${pid$i}"  # プロセスIDを使って待機
done

echo "All validators have completed."

