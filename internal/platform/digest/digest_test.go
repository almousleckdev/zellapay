package digest

import "testing"

func TestSumIsDeterministic(t *testing.T) {
	first, second := Sum("a", "b"), Sum("a", "b")
	if first != second {
		t.Fatal("same input must give the same digest")
	}
}

func TestSumSeparatesFieldBoundaries(t *testing.T) {
	cases := [][2][]string{
		{{"ab", "c"}, {"a", "bc"}},
		{{"a", ""}, {"", "a"}},
		{{"a"}, {"a", ""}},
		{{}, {""}},
	}
	for _, c := range cases {
		if Sum(c[0]...) == Sum(c[1]...) {
			t.Errorf("%q collides with %q", c[0], c[1])
		}
	}
}
