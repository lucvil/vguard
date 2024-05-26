#!/bin/bash

MESSAGE_SIZE=32
NETWORK_DELAY=10

# Loop through the specified range of values
for i in {6..6..2}
do
  # Run the script with the current value as an argument
  ./scripts/run_from_artery.sh $i $MESSAGE_SIZE $NETWORK_DELAY

  python fetch_event.py $i $MESSAGE_SIZE $NETWORK_DELAY

  # Sleep for 5 seconds
  sleep 5

  # Print a message to the console
  echo "Finished iteration $i"

done