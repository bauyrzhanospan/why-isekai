"""Session 1 of the date-formats task: two commits, the second records a dead end."""
FILES_1 = {
    "dates.py": '''import datetime


def parse_date(s):
    """Parse YYYY-MM-DD."""
    return datetime.date.fromisoformat(s.strip())
''',
    "test_dates.py": '''import datetime
import unittest

from dates import parse_date


class T(unittest.TestCase):
    def test_iso(self):
        self.assertEqual(parse_date("2026-11-03"), datetime.date(2026, 11, 3))

    def test_dotted(self):
        self.assertEqual(parse_date("03.11.2026"), datetime.date(2026, 11, 3))

    def test_dotted_spaces(self):
        self.assertEqual(parse_date(" 1.2.2026 "), datetime.date(2026, 2, 1))
''',
}
COMMITS = [
    (FILES_1, "feat: parse ISO dates",
     ["Why: the importer receives ISO dates from the new API"],),
    ({"NOTES.md": "Imports from the bank use DD.MM.YYYY.\n"}, "docs: note bank date format",
     ["Why: bank exports use day-first dates and the importer must read them",
      "Tried: python-dateutil; not installed here and the network is blocked, so the import failed",
      "Next: support DD.MM.YYYY in parse_date with the standard library"],),
]
