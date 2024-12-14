#!/bin/bash

# プロセスIDを格納する配列
pids=()

# スタックサイズの制限を解除
ulimit -s unlimited

rm -rf keys
rm -rf logs
mkdir logs

./scripts/build.sh

PROPOSER_NUM="${1:-3}"
VALIDATOR_NUM="${2:-250}"
MESSAGE_SIZE="${3:-32}"
NETWORK_DELAY="${4:-0}"
VEHICLE_SPEED="${5:-80}"
COM_POSSIBILITY_FLAG="${6:-true}"
ALLOW_BYPASS_FLAG="${7:-true}"
MAIN_PROPOSER_ID="${8:-0}"

PARTICIPANT_NUM=$((PROPOSER_NUM + VALIDATOR_NUM))
PROPOSER_LIST=$(seq -s, 0 $((PROPOSER_NUM - 1)))

# プロポーザの起動
for i in $(seq 0 $((PROPOSER_NUM - 1))); do
  ./scripts/multi_rsu_congestion_run.sh $i 0 $PARTICIPANT_NUM $MESSAGE_SIZE $NETWORK_DELAY $VEHICLE_SPEED $PROPOSER_LIST $COM_POSSIBILITY_FLAG $ALLOW_BYPASS_FLAG $MAIN_PROPOSER_ID&
  pids+=($!)  # プロセスIDを配列に追加
  sleep 0.5
done

# バリデータの起動とプロセスIDの保存
for i in $(seq $PROPOSER_NUM $((PARTICIPANT_NUM - 1))); do
  ./scripts/multi_rsu_congestion_run.sh $i 1 $PARTICIPANT_NUM $MESSAGE_SIZE $NETWORK_DELAY $VEHICLE_SPEED $PROPOSER_LIST $COM_POSSIBILITY_FLAG $ALLOW_BYPASS_FLAG $MAIN_PROPOSER_ID& pid=$!
  pids+=($pid)  # プロセスIDを配列に追加
  sleep 0.1
done

# # trapで子プロセスだけを終了
# trap "kill ${pids[@]}" SIGINT SIGTERM

# 各バリデータの終了を待機
for pid in "${pids[@]}"; do
  wait $pid
  if [ $? -ne 0 ]; then
    echo "Error: Process with PID $pid encountered an error."
    exit 1  # エラー処理
  fi
done

echo "All validators have completed."

