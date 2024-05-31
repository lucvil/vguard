import json
import random

def random_list(n):
    # [1,2,,,19]
    # data_list = list(range(1, 8))
    data = [random.randint(1, 30) for _ in range(n)]
    # data = data_list
    set_data = set(data)
    data = list(set_data)
    # sort
    data.sort()

    return data

# 辞書データを生成
data = {f"{i/100:.2f}": random_list(30) for i in range(0, 100001, 1)}

# JSONファイルに書き込む
with open('data.json', 'w') as f:
    json.dump(data, f, ensure_ascii=False, indent=4)

print("JSONファイルに書き込みが完了しました。")
