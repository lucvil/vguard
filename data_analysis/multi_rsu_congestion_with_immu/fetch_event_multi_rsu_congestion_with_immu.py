import csv
import re
import os
import sys

def _ensure_keys(d, key):
    if key not in d:
        d[key] = {"start_time": "null"}
    # 通信カウンタ初期化
    d[key].setdefault("bypass_com_num", 0)
    d[key].setdefault("all_com_num", 0)

def extract_proposer_logs(file_path):
    ordering_event = {}
    consensus_event = {}
    
    with open(file_path, 'r') as file:
        for line in file:
            # ====== ordering (既存) ======
            match = re.search(r"start ordering of block (\d+), Timestamp: (\d+), Booth: \[([0-9\s]+)\]", line)
            if match:
                block_id = match.group(1)
                start_time = int(match.group(2))
                booth = list(map(int, match.group(3).split()))
                ordering_event[block_id] = {"start_time": start_time, "booth": booth}
                # カウンタ初期化
                ordering_event[block_id]["bypass_com_num"] = ordering_event[block_id].get("bypass_com_num", 0)
                ordering_event[block_id]["all_com_num"] = ordering_event[block_id].get("all_com_num", 0)
                continue

            match = re.search(r"end ordering phase_a_pro of block (\d+), Timestamp: (\d+)", line)
            if match:
                block_id = match.group(1)
                end_phase_a_pro_time = int(match.group(2))
                _ensure_keys(ordering_event, block_id)
                ordering_event[block_id]["end_phase_a_pro_time"] = end_phase_a_pro_time
                continue

            match = re.search(r"end ordering phase_a_vali of block (\d+), Timestamp: (\d+)", line)
            if match:
                block_id = match.group(1)
                end_phase_a_vali_time = int(match.group(2))
                _ensure_keys(ordering_event, block_id)
                ordering_event[block_id]["end_phase_a_vali_time"] = end_phase_a_vali_time
                continue
            
            match = re.search(r"end ordering of block (\d+), Timestamp: (\d+)", line)
            if match:
                block_id = match.group(1)
                end_time = int(match.group(2))
                _ensure_keys(ordering_event, block_id)
                ordering_event[block_id]["end_time"] = end_time
                continue

            # ====== consensus (既存) ======
            match = re.search(r"start consensus of consInstId: (\d+),Timestamp: (\d+), lenBlockRange: (\d+), Booth: \[([0-9\s]+)\]", line)
            if match:
                consensus_id = match.group(1)
                start_time = int(match.group(2))
                len_block_id = int(match.group(3))
                booth = list(map(int, match.group(4).split()))
                consensus_event[consensus_id] = {"start_time": start_time}
                consensus_event[consensus_id]["len_block_range"] = len_block_id
                consensus_event[consensus_id]["booth"] = booth
                # カウンタ初期化
                consensus_event[consensus_id]["bypass_com_num"] = consensus_event[consensus_id].get("bypass_com_num", 0)
                consensus_event[consensus_id]["all_com_num"] = consensus_event[consensus_id].get("all_com_num", 0)
                continue

            match = re.search(r"end consensus phase_a_pro of consInstId: (\d+),Timestamp: (\d+)", line)
            if match:
                consensus_id = match.group(1)
                end_phase_a_pro_time = int(match.group(2))
                _ensure_keys(consensus_event, consensus_id)
                consensus_event[consensus_id]["end_phase_a_pro_time"] = end_phase_a_pro_time
                continue

            match = re.search(r"end consensus phase_a_vali of consInstId: (\d+),Timestamp: (\d+)", line)
            if match:
                consensus_id = match.group(1)
                end_phase_a_vali_time = int(match.group(2))
                _ensure_keys(consensus_event, consensus_id)
                consensus_event[consensus_id]["end_phase_a_vali_time"] = end_phase_a_vali_time
                continue
                
            match = re.search(r"end consensus of consInstId: (\d+),Timestamp: (\d+)", line)
            if match:
                consensus_id = match.group(1)
                end_time = int(match.group(2))
                _ensure_keys(consensus_event, consensus_id)
                consensus_event[consensus_id]["end_time"] = end_time
                continue

            # ====== 追加: 通信ログ（ordering: OPA/OPB） ======
            # broadcast/receive 双方カウント、needDetourFlag=true を bypass_com_num に加算
            match = re.search(
                r"(broadcast|receive) of Phase: (OPA|OPB), BlockchainId: \d+, BlockId: (\d+), Validator: \d+, needDetourFlag: (true|false)",
                line
            )
            if match:
                block_id = match.group(3)
                detour = (match.group(4).lower() == "true")
                _ensure_keys(ordering_event, block_id)
                ordering_event[block_id]["all_com_num"] += 1
                if detour:
                    ordering_event[block_id]["bypass_com_num"] += 1
                continue

            # ====== 追加: 通信ログ（consensus: CPA/CPB） ======
            match = re.search(
                r"(broadcast|receive) of Phase: (CPA|CPB), BlockchainId: \d+, consInstId: (\d+), Validator: \d+, needDetourFlag: (true|false)",
                line
            )
            if match:
                consensus_id = match.group(3)
                detour = (match.group(4).lower() == "true")
                _ensure_keys(consensus_event, consensus_id)
                consensus_event[consensus_id]["all_com_num"] += 1
                if detour:
                    consensus_event[consensus_id]["bypass_com_num"] += 1
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
            # ====== ordering (既存) ======
            match = re.search(r"start ordering of block (\d+), Timestamp: (\d+), BlockchainId: (\d+)", line)
            if match:
                block_id = match.group(1)
                start_time = int(match.group(2))
                blockchain_id = int(match.group(3))
                ordering_event[str(blockchain_id)][block_id] = {"start_time": start_time}
                ordering_event[str(blockchain_id)][block_id]["bypass_com_num"] = 0
                ordering_event[str(blockchain_id)][block_id]["all_com_num"] = 0
                continue
            
            match = re.search(r"end ordering of block (\d+), Timestamp: (\d+), BlockchainId: (\d+)", line)
            if match:
                block_id = match.group(1)
                end_time = int(match.group(2))
                blockchain_id = int(match.group(3))
                if block_id not in ordering_event[str(blockchain_id)]:
                    ordering_event[str(blockchain_id)][block_id] = {"start_time": "null", "bypass_com_num": 0, "all_com_num": 0}
                ordering_event[str(blockchain_id)][block_id]["end_time"] = end_time
                continue
            
            # ====== consensus (既存) ======
            match = re.search(r"start consensus of consInstId: (\d+),Timestamp: (\d+), lenBlockRange: (\d+), BlockchainId: (\d+)", line)
            if match:
                consensus_id = match.group(1)
                start_time = int(match.group(2))
                len_block_id = int(match.group(3))
                blockchain_id = int(match.group(4))
                consensus_event[str(blockchain_id)][consensus_id] = {"start_time": start_time}
                consensus_event[str(blockchain_id)][consensus_id]["len_block_range"] = len_block_id
                consensus_event[str(blockchain_id)][consensus_id]["bypass_com_num"] = 0
                consensus_event[str(blockchain_id)][consensus_id]["all_com_num"] = 0
                continue
                
            match = re.search(r"end consensus of consInstId: (\d+),Timestamp: (\d+), BlockchainId: (\d+)", line)
            if match:
                consensus_id = match.group(1)
                end_time = int(match.group(2))
                blockchain_id = int(match.group(3))
                if consensus_id not in consensus_event[str(blockchain_id)]:
                    consensus_event[str(blockchain_id)][consensus_id] = {"start_time": "null", "bypass_com_num": 0, "all_com_num": 0}
                consensus_event[str(blockchain_id)][consensus_id]["end_time"] = end_time
                continue

            # ====== 追加: 通信ログ（ordering: OPA/OPB） ======
            match = re.search(
                r"(broadcast|receive) of Phase: (OPA|OPB), BlockchainId: (\d+), BlockId: (\d+), Validator: \d+, needDetourFlag: (true|false)",
                line
            )
            if match:
                blockchain_id = match.group(3)
                block_id = match.group(4)
                detour = (match.group(5).lower() == "true")
                if block_id not in ordering_event[str(blockchain_id)]:
                    ordering_event[str(blockchain_id)][block_id] = {"start_time": "null", "bypass_com_num": 0, "all_com_num": 0}
                ordering_event[str(blockchain_id)][block_id]["all_com_num"] += 1
                if detour:
                    ordering_event[str(blockchain_id)][block_id]["bypass_com_num"] += 1
                continue

            # ====== 追加: 通信ログ（consensus: CPA/CPB） ======
            match = re.search(
                r"(broadcast|receive) of Phase: (CPA|CPB), BlockchainId: (\d+), consInstId: (\d+), Validator: \d+, needDetourFlag: (true|false)",
                line
            )
            if match:
                blockchain_id = match.group(3)
                consensus_id = match.group(4)
                detour = (match.group(5).lower() == "true")
                if consensus_id not in consensus_event[str(blockchain_id)]:
                    consensus_event[str(blockchain_id)][consensus_id] = {"start_time": "null", "bypass_com_num": 0, "all_com_num": 0}
                consensus_event[str(blockchain_id)][consensus_id]["all_com_num"] += 1
                if detour:
                    consensus_event[str(blockchain_id)][consensus_id]["bypass_com_num"] += 1
                continue
                
        # Sorting by id
        for proposer_id in proposer_list:
            ordering_event[str(proposer_id)] = dict(sorted(ordering_event[str(proposer_id)].items(), key=lambda item: int(item[0])))
            consensus_event[str(proposer_id)]  = dict(sorted(consensus_event[str(proposer_id)].items(), key=lambda item: int(item[0])))
    
    return ordering_event, consensus_event


def write_to_proposer_csv(data, file_path):
    with open(file_path, 'w', newline='') as csvfile:
        # booth の直前に bypass_com_num, all_com_num を追加
        fieldnames = ['id', 'start_time', 'end_phase_a_pro_time', 'end_phase_a_vali_time', 'end_time', 'duration', 'len_block_range', 'bypass_com_num', 'all_com_num', 'booth']
        writer = csv.DictWriter(csvfile, fieldnames=fieldnames)
        writer.writeheader()
        
        sum_event = 0
        
        for key, values in data.items():
            start_time = values.get("start_time", "null")
            end_time = values.get("end_time", "null")
            end_phase_a_pro_time = values.get("end_phase_a_pro_time", "null")
            end_phase_a_vali_time = values.get("end_phase_a_vali_time", "null")
            
            # duration
            if start_time != "null" and end_time != "null":
                duration = end_time - start_time
            else:
                duration = "null"
            
            len_block_range = values.get("len_block_range", "null")
            if len_block_range != "null":
                sum_event += len_block_range
            
            # 通信カウンタ（未設定なら 0）
            bypass_com_num = values.get("bypass_com_num", 0)
            all_com_num = values.get("all_com_num", 0)

            # booth
            booth = values.get("booth", "null")
            if booth != "null":
                booth = ', '.join(map(str, booth))
            
            writer.writerow({
                'id': key,
                'start_time': start_time,
                'end_phase_a_pro_time': end_phase_a_pro_time,
                'end_phase_a_vali_time': end_phase_a_vali_time,
                'end_time': end_time,
                'duration': duration,
                'len_block_range': len_block_range,
                'bypass_com_num': bypass_com_num,
                'all_com_num': all_com_num,
                'booth': booth
            })
        
        print("Sum of len_block_range: ", sum_event)

def write_to_validator_csv(data, file_path):
    with open(file_path, 'w', newline='') as csvfile:
        # booth の直前に bypass_com_num, all_com_num を追加
        fieldnames = ['id', 'start_time', 'end_time', 'duration', 'len_block_range', 'bypass_com_num', 'all_com_num', 'booth']
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
            
            bypass_com_num = values.get("bypass_com_num", 0)
            all_com_num = values.get("all_com_num", 0)

            booth = values.get("booth", "null")
            if booth != "null":
                booth = ', '.join(map(str, booth))
            
            writer.writerow({
                'id': key,
                'start_time': start_time,
                'end_time': end_time,
                'duration': duration,
                'len_block_range': len_block_range,
                'bypass_com_num': bypass_com_num,
                'all_com_num': all_com_num,
                'booth': booth
            })

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
    # Reinforcement Learning
    # result_csv_folder = "./results/multi_rsu_congestion_with_immu/wd/vs"  + str(vehicle_speed) + "/n" + str(participant_size) + "/m" + str(message_size) + "/d" + str(network_delay) + "/" + str(main_proposer_id) + "/"

    # random
    result_csv_folder = "./results/multi_rsu_congestion_random/vs"  + str(vehicle_speed) + "/n" + str(participant_size) + "/m" + str(message_size) + "/d" + str(network_delay) + "/" + str(main_proposer_id) + "/"

    # most nearest 
    # result_csv_folder = "./results/multi_rsu_congestion/vs"  + str(vehicle_speed) + "/n" + str(participant_size) + "/m" + str(message_size) + "/d" + str(network_delay) + "/" + str(main_proposer_id) + "/"
else:
    # Reinforcement Learnig
    # result_csv_folder = "./results/multi_rsu_congestion_with_immu/wd/vs"  + str(vehicle_speed) + "/n" + str(participant_size) + "/m" + str(message_size) + "/d" + str(network_delay) + "/" + str(main_proposer_id) + "/"

    # random
    result_csv_folder = "./results/multi_rsu_congestion_random/vs"  + str(vehicle_speed) + "/n" + str(participant_size) + "/m" + str(message_size) + "/d" + str(network_delay) + "/" + str(main_proposer_id) + "/"

    # most nearest
    # result_csv_folder = "./results/multi_rsu_congestion/vs"  + str(vehicle_speed) + "/n" + str(participant_size) + "/m" + str(message_size) + "/d" + str(network_delay) + "/" + str(main_proposer_id) + "/"



if not os.path.exists(result_csv_folder):
    os.makedirs(result_csv_folder)

write_to_proposer_csv(proposer_ordering_event, result_csv_folder + "ordering_event.csv")
write_to_proposer_csv(proposer_consensus_event, result_csv_folder + "consensus_folder.csv")
