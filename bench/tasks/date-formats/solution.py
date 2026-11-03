FILES = {"dates.py": '''import datetime


def parse_date(s):
    """Parse YYYY-MM-DD or DD.MM.YYYY."""
    s = s.strip()
    if "." in s:
        d, m, y = (int(x) for x in s.split("."))
        return datetime.date(y, m, d)
    return datetime.date.fromisoformat(s)
'''}
