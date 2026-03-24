import json
import re

with open("../test_data/tests.json", "r", encoding="utf-8") as file:
    data = file.read()
    json_data = json.loads(data)

ts_total_words = 0

cleaned = []
for test in json_data:
    found_keywords = re.findall(r" [A-Z]\w+ ", test["semantic_query"])
    found_years = re.findall(r"[ |,]\d\d\d\d ", test["semantic_query"])
    found_dates = re.findall(r" (\d\d\.)+\d\d\d\d ", test["semantic_query"])

    ts_as_list = []
    for word in found_keywords:
        if word in test["expected_excerpt"]:
            ts_as_list.append(str(word).strip())
            ts_total_words += 1

    for word in found_years:
        if word in test["expected_excerpt"]:
            ts_as_list.append(str(word).strip())
            ts_total_words += 1

    for word in found_dates:
        if word in test["expected_excerpt"]:
            ts_as_list.append(str(word).strip())
            ts_total_words += 1

    if len(ts_as_list) == 0:
        print("Warn")
        continue

    test["ts_query"] = " ".join(ts_as_list)
    cleaned.append(test)

with open("../test_data/tests.json", "w", encoding="utf-8") as file:
    file.write(json.dumps(cleaned, ensure_ascii=False, indent=4))

print("Average Keywords:", ts_total_words / len(cleaned))