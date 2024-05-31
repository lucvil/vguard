import csv
import re
import os
import sys

def extract_logs(file_path):
    ordering_event = {}
    consensus_event = {}
    
    with open(file_path, 'r') as file:
        for line in file:
            # Extracting Id and Timestamp from the log line
            match = re.search(r"start ordering of block (\d+), Timestamp: (\d+)", line)
            if match:
                block_id = match.group(1)
                start_time = int(match.group(2))
                ordering_event[block_id] = {"start_time": start_time}
                continue
            
            match = re.search(r"end ordering of block (\d+), Timestamp: (\d+)", line)
            if match:
                block_id = match.group(1)
                end_time = int(match.group(2))
                if block_id not in ordering_event:
                    ordering_event[block_id] = {"start_time": "null"}
                ordering_event[block_id]["end_time"] = end_time
                continue
            
            match = re.search(r"start consensus of consInstId: (\d+),Timestamp: (\d+), lenBlockRange: (\d+)", line)
            if match:
                consensus_id = match.group(1)
                start_time = int(match.group(2))
                len_block_id = int(match.group(3))
                consensus_event[consensus_id] = {"start_time": start_time}
                consensus_event[consensus_id]["len_block_range"] = len_block_id
                continue
                
            match = re.search(r"end consensus of consInstId: (\d+),Timestamp: (\d+)", line)
            if match:
                consensus_id = match.group(1)
                end_time = int(match.group(2))
                if consensus_id not in consensus_event:
                    consensus_event[consensus_id] = {"start_time": "null"}
                consensus_event[consensus_id]["end_time"] = end_time
                continue
                
        # Sorting by id
        ordering_event = dict(sorted(ordering_event.items(), key=lambda item: int(item[0])))
        consensus_event = dict(sorted(consensus_event.items(), key=lambda item: int(item[0])))
    
    return ordering_event, consensus_event

def write_to_csv(data, file_path):
    with open(file_path, 'w', newline='') as csvfile:
        fieldnames = ['id', 'start_time', 'end_time', 'duration', 'len_block_range']
        writer = csv.DictWriter(csvfile, fieldnames=fieldnames)
        writer.writeheader()
        
        sum_event = 0
        
        for key, values in data.items():
            start_time = values.get("start_time", "null")
            end_time = values.get("end_time", "null")
            
            if start_time != "null" and end_time != "null":
                duration = end_time - start_time
            else:
                duration = "null"
            len_block_range = values.get("len_block_range", "null")
            if len_block_range != "null":
                sum_event += len_block_range
            
            writer.writerow({'id': key, 'start_time': start_time, 'end_time': end_time, 'duration': duration, 'len_block_range': len_block_range})
            
        print("Sum of len_block_range: ", sum_event)


arguments = sys.argv[1:]

participant_size = int(arguments[0])
booth_size = participant_size
message_size = int(arguments[1])
network_delay = int(arguments[2])

try:
    vehicle_speed = int(arguments[3])
except IndexError:
    print("Error: Not enough arguments provided.")
    vehicle_speed = None  # またはデフォルト値を設定する

log_file = "./logs/s0/n_" + str(participant_size) + "_b100_d" + str(network_delay) + ".log"
ordering_event, consensus_event = extract_logs(log_file)

# print(ordering_event)

# make directory to store result csv
if vehicle_speed is None:
    result_csv_folder = "./results/booth_change/others/n" + str(participant_size) + "/m" + str(message_size) + "/d" + str(network_delay) + "/"
else:
    result_csv_folder = "./results/booth_change/vs"  + str(vehicle_speed) + "/n" + str(participant_size) + "/m" + str(message_size) + "/d" + str(network_delay) + "/"
# result_csv_folder = "./results/boo21/"
if not os.path.exists(result_csv_folder):
    os.makedirs(result_csv_folder)

# sum result_csv_folder last row

write_to_csv(ordering_event, result_csv_folder + "ordering_event.csv")
write_to_csv(consensus_event, result_csv_folder + "consensus_event.csv")