import csv
import re
import os
import sys

def extract_proposer_logs(file_path):
    ordering_event = {}
    consensus_event = {}
    
    with open(file_path, 'r') as file:
        for line in file:
            # Extracting Id and Timestamp from the log line
            match = re.search(r"start ordering of block (\d+), Timestamp: (\d+), Booth: \[([0-9\s]+)\]", line)
            if match:
                block_id = match.group(1)
                start_time = int(match.group(2))
                booth = list(map(int, match.group(3).split()))  # Booth の数値をリストに変換
                ordering_event[block_id] = {"start_time": start_time, "booth": booth}
                continue
            
            match = re.search(r"end ordering of block (\d+), Timestamp: (\d+)", line)
            if match:
                block_id = match.group(1)
                end_time = int(match.group(2))
                if block_id not in ordering_event:
                    ordering_event[block_id] = {"start_time": "null"}
                ordering_event[block_id]["end_time"] = end_time
                continue
            
            match = re.search(r"start consensus of consInstId: (\d+),Timestamp: (\d+), lenBlockRange: (\d+), Booth: \[([0-9\s]+)\]", line)
            if match:
                consensus_id = match.group(1)
                start_time = int(match.group(2))
                len_block_id = int(match.group(3))
                booth = list(map(int, match.group(4).split())) 
                consensus_event[consensus_id] = {"start_time": start_time}
                consensus_event[consensus_id]["len_block_range"] = len_block_id
                consensus_event[consensus_id]["booth"] = booth
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

def extract_validator_logs(file_path, proposer_list):
    ordering_event = {}
    consensus_event = {}
    for proposer_id in proposer_list:
        ordering_event[str(proposer_id)] = {}
        consensus_event[str(proposer_id)] = {}

    
    with open(file_path, 'r') as file:
        for line in file:
            # Extracting Id and Timestamp from the log line
            match = re.search(r"start ordering of block (\d+), Timestamp: (\d+), BlockchainId: (\d+)", line)
            if match:
                block_id = match.group(1)
                start_time = int(match.group(2))
                blockchain_id = int(match.group(3))
                ordering_event[str(blockchain_id)][block_id] = {"start_time": start_time}
                continue
            
            match = re.search(r"end ordering of block (\d+), Timestamp: (\d+), BlockchainId: (\d+)", line)
            if match:
                block_id = match.group(1)
                end_time = int(match.group(2))
                blockchain_id = int(match.group(3))
                if block_id not in ordering_event[str(blockchain_id)]:
                    ordering_event[str(blockchain_id)][block_id] = {"start_time": "null"}
                ordering_event[str(blockchain_id)][block_id]["end_time"] = end_time
                continue
            
            match = re.search(r"start consensus of consInstId: (\d+),Timestamp: (\d+), lenBlockRange: (\d+), BlockchainId: (\d+)", line)
            if match:
                consensus_id = match.group(1)
                start_time = int(match.group(2))
                len_block_id = int(match.group(3))
                blockchain_id = int(match.group(4))
                consensus_event[str(blockchain_id)][consensus_id] = {"start_time": start_time}
                consensus_event[str(blockchain_id)][consensus_id]["len_block_range"] = len_block_id
                continue
                
            match = re.search(r"end consensus of consInstId: (\d+),Timestamp: (\d+), BlockchainId: (\d+)", line)
            if match:
                consensus_id = match.group(1)
                end_time = int(match.group(2))
                blockchain_id = int(match.group(3))
                if consensus_id not in consensus_event[str(blockchain_id)]:
                    consensus_event[str(blockchain_id)][consensus_id] = {"start_time": "null"}
                consensus_event[str(blockchain_id)][consensus_id]["end_time"] = end_time
                continue
                
        # Sorting by id
        for proposer_id in proposer_list:
            ordering_event[str(proposer_id)] = dict(sorted(ordering_event[str(proposer_id)].items(), key=lambda item: int(item[0])))
            consensus_event[str(proposer_id)]  = dict(sorted(consensus_event[str(proposer_id)].items(), key=lambda item: int(item[0])))
    
    return ordering_event, consensus_event


def write_to_proposer_csv(data, file_path):
    with open(file_path, 'w', newline='') as csvfile:
        # booth 列を追加
        fieldnames = ['id', 'start_time', 'end_time', 'duration', 'len_block_range', 'booth']
        writer = csv.DictWriter(csvfile, fieldnames=fieldnames)
        writer.writeheader()
        
        sum_event = 0
        
        for key, values in data.items():
            start_time = values.get("start_time", "null")
            end_time = values.get("end_time", "null")
            
            # duration の計算
            if start_time != "null" and end_time != "null":
                duration = end_time - start_time
            else:
                duration = "null"
            
            len_block_range = values.get("len_block_range", "null")
            if len_block_range != "null":
                sum_event += len_block_range
            
            # booth の処理
            booth = values.get("booth", "null")
            if booth != "null":
                booth = ', '.join(map(str, booth))  # booth をカンマ区切りの文字列に変換
            
            # CSV に書き込み
            writer.writerow({
                'id': key,
                'start_time': start_time,
                'end_time': end_time,
                'duration': duration,
                'len_block_range': len_block_range,
                'booth': booth  # booth を書き込み
            })
        
        print("Sum of len_block_range: ", sum_event)

def write_to_validator_csv(data, file_path):
    with open(file_path, 'w', newline='') as csvfile:
        # booth 列を追加
        fieldnames = ['id', 'start_time', 'end_time', 'duration', 'len_block_range', 'booth']
        writer = csv.DictWriter(csvfile, fieldnames=fieldnames)
        writer.writeheader()
        
        sum_event = 0
        
        for key, values in data.items():
            start_time = values.get("start_time", "null")
            end_time = values.get("end_time", "null")
            
            # duration の計算
            if start_time != "null" and end_time != "null":
                duration = end_time - start_time
            else:
                duration = "null"
            
            len_block_range = values.get("len_block_range", "null")
            if len_block_range != "null":
                sum_event += len_block_range
            
            # booth の処理
            booth = values.get("booth", "null")
            if booth != "null":
                booth = ', '.join(map(str, booth))  # booth をカンマ区切りの文字列に変換
            
            # CSV に書き込み
            writer.writerow({
                'id': key,
                'start_time': start_time,
                'end_time': end_time,
                'duration': duration,
                'len_block_range': len_block_range,
                'booth': booth  # booth を書き込み
            })

            # print({
            #     'id': key,
            #     'start_time': start_time,
            #     'end_time': end_time,
            #     'duration': duration,
            #     'len_block_range': len_block_range,
            #     'booth': booth  # booth を書き込み
            # })


arguments = sys.argv[1:]
proposer_num = int(arguments[0])
validator_num = int(arguments[1])
participant_size = proposer_num + validator_num
booth_size = participant_size
message_size = int(arguments[2])
network_delay = int(arguments[3])
vehicle_speed = int(arguments[4])
com_possibility_flag = True
allow_bypass_flag = arguments[5].lower() == 'true'
main_proposer_id = int(arguments[6])

proposer_list = list(range(proposer_num))
validator_list = list(range(proposer_num, participant_size))

print(allow_bypass_flag)


log_file = "./logs/s" +  str(main_proposer_id) + "/n_" + str(participant_size) + "_b100_d" + str(network_delay) + ".log"
proposer_ordering_event, proposer_consensus_event = extract_proposer_logs(log_file)

if allow_bypass_flag:
    result_csv_folder = "./results/multi_rsu_congestion_fixed_vehicle_num/fixed_v20/n" + str(participant_size) + "/m" + str(message_size) + "/d" + str(network_delay) + "/" + str(main_proposer_id) + "/"
else:
    result_csv_folder = "./results/multi_rsu_congestion_fixed_vehicle_num/fixed_v20/n" + str(participant_size) + "/m" + str(message_size) + "/d" + str(network_delay) + "/" + str(main_proposer_id) + "/"

if not os.path.exists(result_csv_folder):
    os.makedirs(result_csv_folder)

write_to_proposer_csv(proposer_ordering_event, result_csv_folder + "ordering_event.csv")
write_to_proposer_csv(proposer_consensus_event, result_csv_folder + "consensus_folder.csv")



