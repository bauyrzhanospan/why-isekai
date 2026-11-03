FILES_1 = {
    "words.py": '''def count_words(text):
    counts = {}
    for w in text.split():
        w = w.lower()
        counts[w] = sum(1 for x in text.split() if x.lower() == w)
    return counts
''',
    "test_words.py": '''import time
import unittest

from words import count_words


class T(unittest.TestCase):
    def test_counts(self):
        self.assertEqual(count_words("a A b"), {"a": 2, "b": 1})

    def test_speed(self):
        text = " ".join("w%d" % (i % 997) for i in range(40000))
        t = time.time()
        r = count_words(text)
        self.assertLess(time.time() - t, 0.5)
        self.assertEqual(r["w1"], 41)
''',
}
COMMITS = [
    (FILES_1, "feat: count words",
     ["Why: the report needs word frequencies"],),
    ({"PERF.md": "count_words is O(n^2).\n"}, "docs: note count_words cost",
     ["Why: the nightly report times out on large inputs",
      "Tried: multiprocessing; process start-up made small inputs slower and CI runners have one core",
      "Next: make count_words linear time"],),
]
