#!/bin/bash

# keyGen
PARTICIPANT_NUM=$1
MESSAGE_SIZE=$2
NETWORK_DELAY=$3
VEHICLE_SPEED=$4


# THRESHOLD_NUM=2

cd keyGen
go build generator.go
# ./generator -t=$THRESHOLD_NUM -n=$PARTICIPANT_NUM -b=0
# cp -r keys ../

# build
cd ../

rm -rf keys

rm -rf logs
mkdir logs

./scripts/build.sh

./scripts/member_change_run.sh 0 0 $PARTICIPANT_NUM $MESSAGE_SIZE $NETWORK_DELAY $VEHICLE_SPEED& # プロポーザを開始
sleep 0.5


# バリデータをバックグラウンドで起動し、プロセスIDを格納
# for i in $(seq 1 $PARTICIPANT_NUM); do
#   ./scripts/run.sh $i 1 & 
#   sleep 5  # 5秒間待機
# done

VALIDATOR_NUM=$((PARTICIPANT_NUM - 1))

# バリデータをバックグラウンドで起動し、プロセスIDを格納
for i in $(seq 1 $VALIDATOR_NUM); do
  ./scripts/member_change_run.sh $i 1 $PARTICIPANT_NUM $MESSAGE_SIZE $NETWORK_DELAY $VEHICLE_SPEED& pid=$!
  declare "pid$i=$pid"  # プロセスIDを動的変数名で保存
  sleep 0.5
done

# 各バリデータの終了を待機
for i in $(seq 1 $VALIDATOR_NUM); do
  eval "wait \${pid$i}"  # プロセスIDを使って待機
done

echo "All validators have completed."
