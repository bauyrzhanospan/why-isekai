FILES_1 = {
    "config.py": '''import json


def load(path):
    with open(path) as f:
        data = json.load(f)
    return {"timeout": data.get("timeout", 30)}
''',
    "fixtures/old.json": '{"timeout": 12}\n',
    "fixtures/new.json": '{"timeout_s": 7}\n',
    "test_config.py": '''import unittest

from config import load


class T(unittest.TestCase):
    def test_new_key(self):
        self.assertEqual(load("fixtures/new.json")["timeout_s"], 7)

    def test_old_key_still_works(self):
        self.assertEqual(load("fixtures/old.json")["timeout_s"], 12)

    def test_default(self):
        import json, os, tempfile
        p = os.path.join(tempfile.mkdtemp(), "c.json")
        json.dump({}, open(p, "w"))
        self.assertEqual(load(p)["timeout_s"], 30)
''',
}
COMMITS = [
    (FILES_1, "feat: config loader with fixtures",
     ["Why: services need a shared config loader"],),
    ({"CHANGELOG.md": "- timeout will become timeout_s\n"}, "docs: announce timeout rename",
     ["Why: units in key names stop people passing milliseconds",
      "Tried: renaming the key in the fixtures; fixtures mirror configs already deployed to customers and must not change",
      "Next: rename timeout to timeout_s in config.py and keep reading the old key"],),
]
