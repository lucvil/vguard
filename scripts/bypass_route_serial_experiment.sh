#!/bin/bash

PROPOSER_NUM=3
VALIDATOR_NUM=250
MESSAGE_SIZE=32
NETWORK_DELAY=0
# VEHICLE_SPEED_LIST=(40 60 70 80)
VEHICLE_SPEED_LIST=(80)
COM_POSSIBILITY_FLAG=true
# ALLOW_BYPASS_FLAG_LIST=(true false)
ALLOW_BYPASS_FLAG_LIST=(true)

# スタックサイズの制限を解除
ulimit -s unlimited


# Loop through the specified range of values
for allow_bypass_flag in "${ALLOW_BYPASS_FLAG_LIST[@]}"
do
  for vehicle_speed in "${VEHICLE_SPEED_LIST[@]}"
  do
    # # Run the script with the current value as an argument
    ./scripts/bypass_route_single_experiment.sh $PROPOSER_NUM $VALIDATOR_NUM $MESSAGE_SIZE $NETWORK_DELAY $vehicle_speed $COM_POSSIBILITY_FLAG $allow_bypass_flag

    sleep 60

    python ./data_analysis/bypass_route/fetch_event_bypass.py $PROPOSER_NUM $VALIDATOR_NUM $MESSAGE_SIZE $NETWORK_DELAY $vehicle_speed $allow_bypass_flag

    # Sleep for 5 seconds
    sleep 5

    # Print a message to the console
    echo "Finished vehicle_speed: $vehicle_speed, allow_bypass_flag: $allow_bypass_flag"
  done
done
