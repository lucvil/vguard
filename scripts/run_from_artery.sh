#!/bin/bash

# keyGen
PARTICIPANT_NUM=$1
BOOTH_SIZE=$PARTICIPANT_NUM
# BOOTH_SIZE=6
MESSAGE_SIZE=$2
THRESHOLD_NUM=$((BOOTH_SIZE/3 * 2))
NETWORK_DELAY=$3


# THRESHOLD_NUM=2

# cd keyGen
# go build generator.go
# ./generator -t=$THRESHOLD_NUM -n=$PARTICIPANT_NUM
# cp -r keys ../

# build
# cd ../
./scripts/build.sh

# make log folder
rm -rf logs_from_artery
mkdir logs_from_artery

./scripts/run.sh 0 0 $PARTICIPANT_NUM $BOOTH_SIZE $MESSAGE_SIZE $NETWORK_DELAY & # プロポーザを開始
sleep 3


# バリデータをバックグラウンドで起動し、プロセスIDを格納
# for i in $(seq 1 $PARTICIPANT_NUM); do
#   ./scripts/run.sh $i 1 & 
#   sleep 5  # 5秒間待機
# done

VALIDATOR_NUM=$((PARTICIPANT_NUM - 1))

# バリデータをバックグラウンドで起動し、プロセスIDを格納
for i in $(seq 1 $VALIDATOR_NUM); do
  ./scripts/run.sh $i 1 $PARTICIPANT_NUM $BOOTH_SIZE $MESSAGE_SIZE $NETWORK_DELAY & pid=$!
  declare "pid$i=$pid"  # プロセスIDを動的変数名で保存
  sleep 3
done

# 各バリデータの終了を待機
for i in $(seq 1 $VALIDATOR_NUM); do
  eval "wait \${pid$i}"  # プロセスIDを使って待機
done

echo "All validators have completed."
