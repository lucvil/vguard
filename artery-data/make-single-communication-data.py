import json

# 提案者リストとバリデータリストの設定
proposer_list = [0, 1]
validator_list = [2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12]

# communication-data.jsonを読み込む
with open('communication-data.json', 'r') as file:
    communication_data = json.load(file)

# proposer_listに基づいてデータを分割して新しい辞書を作成
output_data = {proposer: {} for proposer in proposer_list}

for key, value in communication_data.items():
    for proposer in proposer_list:
        output_data[proposer][key] = value[str(proposer)]

# communication-data-0.json と communication-data-1.json にデータを保存
for proposer in proposer_list:
    filename = f'communication-data-{proposer}.json'
    with open(filename, 'w') as file:
        json.dump(output_data[proposer], file, indent=4)

# validator_listに基づいてバリデータの通信可能なproposerリストを作成
validator_output_data = {validator: {} for validator in validator_list}

for key, value in communication_data.items():
    for validator in validator_list:
        # 各バリデータに対して通信可能なプロポーザーリストを収集
        communication_list = []
        for proposer in proposer_list:
            # プロポーザーのリストにバリデータが含まれているかをチェック
            if validator in value[str(proposer)]:
                communication_list.append(proposer)
        
        # 通信可能なプロポーザーリストを順番通りに保存
        validator_output_data[validator][key] = communication_list

# communication-data-2.json から communication-data-12.json にデータを保存
for validator in validator_list:
    filename = f'communication-data-{validator}.json'
    with open(filename, 'w') as file:
        json.dump(validator_output_data[validator], file, indent=4)

print("communication-data-2.json から communication-data-12.json が作成されました。")
