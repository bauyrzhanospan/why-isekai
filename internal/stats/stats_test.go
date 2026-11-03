package stats

import "testing"

func TestLevelFor(t *testing.T) {
	cases := []struct{ xp, level, next, pct int }{
		{0, 1, 100, 0}, {99, 1, 100, 99}, {100, 2, 250, 0}, {450, 3, 500, 80}, {650, 4, 800, 50}, {5000, 9, 3800, 100},
	}
	for _, c := range cases {
		l := LevelFor(c.xp)
		if l.Level != c.level || l.NextXP != c.next || (c.xp < 3000 && l.Pct != c.pct) {
			t.Errorf("xp %d: %+v", c.xp, l)
		}
	}
}
