import json
import random

# データ生成に関する設定
start_time = 100.00
end_time = 300.99
increment = 0.01

# 生成するリストの値
list1_values = [2,3,4,5,6]
list2_values = [7,8,9,10,11]

# 出力用のデータを保持する辞書
data = {}

current_time = start_time

# 0.01秒刻みでデータを生成
while current_time <= end_time:
    # 時たまリスト間で値を入れ替える
    if random.random() < 0.01:  # 1%の確率で値を入れ替える
        swap_value = random.choice(list1_values)
        list1_values.remove(swap_value)
        list2_values.append(swap_value)
        
        swap_back_value = random.choice(list2_values)
        list2_values.remove(swap_back_value)
        list1_values.append(swap_back_value)

    # 辞書データのkeyは0と1で固定
    data["{:.2f}".format(round(current_time, 2))] = {
        0: random.sample(list1_values, len(list1_values)),  # list1_values のランダムな並び
        1: random.sample(list2_values, len(list2_values))   # list2_values のランダムな並び
    }

    # 0.01秒刻みでデータを変化させる
    current_time = round(current_time + increment, 2)

# JSONファイルとして保存
with open("communication-data.json", "w") as json_file:
    json.dump(data, json_file, indent=4)

print("データが 'vehicle_time_data.json' に保存されました。")
