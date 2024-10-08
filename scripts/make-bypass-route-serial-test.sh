#!/bin/bash

# 子プロセスが終了する際に全ての子プロセスも終了するようにする
trap "kill 0" EXIT

./scripts/build.sh

PARTICIPANT_NUM=12

./scripts/member_change_run_test.sh 0 0 $PARTICIPANT_NUM& # プロポーザを開始
sleep 0.5
./scripts/member_change_run_test.sh 1 0 $PARTICIPANT_NUM& # プロポーザを開始
sleep 0.5



# バリデータをバックグラウンドで起動し、プロセスIDを格納
for i in $(seq 2 $((PARTICIPANT_NUM - 1))); do
  ./scripts/member_change_run_test.sh $i 1 $PARTICIPANT_NUM& pid=$!
  declare "pid$i=$pid"  # プロセスIDを動的変数名で保存
  sleep 0.1
done

# 各バリデータの終了を待機
for i in $(seq 2 $((PARTICIPANT_NUM - 1))); do
  eval "wait \${pid$i}"  # プロセスIDを使って待機
done

echo "All validators have completed."

