#!/bin/bash

PROPOSER_NUM=3
VALIDATOR_NUM=160
MESSAGE_SIZE=32
NETWORK_DELAY=0
VEHICLE_SPEED=80
COM_POSSIBILITY_FLAG=true
ALLOW_BYPASS_FLAG=true
# MAIN_PROPOSER_LIST=$(seq -s, 0 $((PROPOSER_NUM - 1)))
MAIN_PROPOSER_LIST=(0 1 2)

# スタックサイズの制限を解除
ulimit -s unlimited

# Loop through the specified range of values
for main_proposer in "${MAIN_PROPOSER_LIST[@]}"
do
  # Run the script with the current value as an argument
  ./scripts/multi_rsu_single_street_congestion_with_immu_single_experiment.sh $PROPOSER_NUM $VALIDATOR_NUM $MESSAGE_SIZE $NETWORK_DELAY $VEHICLE_SPEED $COM_POSSIBILITY_FLAG $ALLOW_BYPASS_FLAG $main_proposer

  # Sleep for 60 seconds
  sleep 60

  # Run the data analysis script
  python ./data_analysis/multi_rsu_single_street_congestion_with_immu/fetch_event_multi_rsu_congestion_with_immu.py $PROPOSER_NUM $VALIDATOR_NUM $MESSAGE_SIZE $NETWORK_DELAY $VEHICLE_SPEED $ALLOW_BYPASS_FLAG $main_proposer

  # Sleep for 5 seconds
  sleep 5

  # Print a message to the console
  echo "Finished vehicle_speed: $VEHICLE_SPEED, allow_bypass_flag: $ALLOW_BYPASS_FLAG"
done
