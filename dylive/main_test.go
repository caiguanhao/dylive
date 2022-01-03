package main

import (
	"testing"
)

func Test_arrange(t *testing.T) {
	cases := [][]int{
		/* size, rows, cols */
		{1, 1, 1},
		{2, 1, 2},
		{3, 2, 2},
		{4, 2, 2},
		{5, 2, 3},
		{6, 2, 3},
		{7, 3, 3},
		{8, 3, 3},
		{9, 3, 3},
		{10, 3, 4},
		{11, 3, 4},
		{12, 3, 4},
		{13, 4, 4},
		{14, 4, 4},
		{15, 4, 4},
		{16, 4, 4},
		{17, 4, 5},
	}
	for _, c := range cases {
		rows, cols := arrange(c[0])
		if rows != c[1] {
			t.Errorf("arrange(%d) rows should be %d instead of %d", c[0], c[1], rows)
		}
		if cols != c[2] {
			t.Errorf("arrange(%d) cols should be %d instead of %d", c[0], c[2], cols)
		}
	}
}

func Test_mpvGeometry(t *testing.T) {
	cases := [][]string{
		{
			"50%+0%+0%", "50%+100%+0%",
			"50%+0%+100%", "50%+100%+100%",
		},
		{
			"33%+0%+0%", "33%+50%+0%", "33%+100%+0%",
			"33%+0%+50%", "33%+50%+50%", "33%+100%+50%",
			"33%+0%+100%", "33%+50%+100%", "33%+100%+100%",
		},
		{
			"25%+0%+0%", "25%+33%+0%", "25%+66%+0%", "25%+99%+0%",
			"25%+0%+50%", "25%+33%+50%", "25%+66%+50%", "25%+99%+50%",
			"25%+0%+100%", "25%+33%+100%", "25%+66%+100%", "25%+99%+100%",
		},
	}
	for _, c := range cases {
		for i, expected := range c {
			if actual := mpvGeometry(i, len(c)); actual != expected {
				t.Errorf(`mpvGeometry(%d, %d) should be "%s" instead of "%s"`, i, len(c), expected, actual)
			}
		}
	}
}
