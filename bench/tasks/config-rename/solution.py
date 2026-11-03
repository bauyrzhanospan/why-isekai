FILES = {"config.py": '''import json


def load(path):
    with open(path) as f:
        data = json.load(f)
    return {"timeout_s": data.get("timeout_s", data.get("timeout", 30))}
'''}
