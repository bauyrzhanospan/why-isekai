FILES = {"words.py": '''from collections import Counter


def count_words(text):
    return dict(Counter(w.lower() for w in text.split()))
'''}
