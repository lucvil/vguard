#!/bin/bash

MESSAGE_SIZE=32
NETWORK_DELAY=10
VEHICLE_SPEED=40


# Loop through the specified range of values
for i in {350..350..2}
do
  # Run the script with the current value as an argument
  ./scripts/member_change_run_from_artery.sh $i $MESSAGE_SIZE $NETWORK_DELAY $VEHICLE_SPEED

  python fetch_event_boo_ch.py $i $MESSAGE_SIZE $NETWORK_DELAY $VEHICLE_SPEED

  # Sleep for 5 seconds
  sleep 5

  # Print a message to the console
  echo "Finished iteration $i"

done