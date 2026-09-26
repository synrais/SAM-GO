package utils

import (
	"reflect"
	"testing"
)

func TestListZip(t *testing.T) {
	contents, err := ListZip("testdata/test.zip")
	if err != nil {
		t.Error(err)
	}
	if len(contents) != 5 {
		t.Errorf("got %d files, want 5", len(contents))
	}
	if contents[0] != "f1" {
		t.Errorf("got %q, want f1", contents[0])
	}
	if _, err = ListZip("testdata/not_zip.zip"); err == nil {
		t.Error("expected error for non-zip file")
	}
}

func TestMin(t *testing.T) {
	var tests = []struct {
		xs   []int
		want int
	}{
		{[]int{}, 0},
		{[]int{1}, 1},
		{[]int{1, 2}, 1},
		{[]int{2, 1}, 1},
		{[]int{1, 2, 3}, 1},
		{[]int{3, 2, 1}, 1},
		{[]int{1, 2, 3, 4}, 1},
		{[]int{4, 2, 1, 3}, 1},
	}
	for _, tt := range tests {
		if got := Min(tt.xs); got != tt.want {
			t.Errorf("Min(%v) = %v, want %v", tt.xs, got, tt.want)
		}
	}
}

func TestMapKeys(t *testing.T) {
	// FIXME: this shouldn't be checking order of result
	var tests = []struct {
		m    map[string]int
		want []string
	}{
		{map[string]int{}, []string{}},
		{map[string]int{"a": 1}, []string{"a"}},
		{map[string]int{"a": 1, "b": 2}, []string{"a", "b"}},
	}
	for _, tt := range tests {
		if got := MapKeys(tt.m); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("MapKeys(%v) = %v, want %v", tt.m, got, tt.want)
		}
	}
}
