package service

import (
	"reflect"
	"testing"

	"github.com/qq900306ss/badminton-court-api/internal/model"
)

func TestNormPlaying(t *testing.T) {
	cases := []struct {
		in   []string
		want []string
	}{
		{nil, []string{"", "", "", ""}},
		{[]string{"a"}, []string{"a", "", "", ""}},
		{[]string{"a", "b", "c", "d"}, []string{"a", "b", "c", "d"}},
		{[]string{"a", "b", "c", "d", "e"}, []string{"a", "b", "c", "d"}}, // truncate
	}
	for _, c := range cases {
		if got := normPlaying(c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("normPlaying(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestPlayingCountAndClearSlot(t *testing.T) {
	p := []string{"a", "", "c", ""}
	if n := playingCount(p); n != 2 {
		t.Errorf("playingCount = %d, want 2", n)
	}
	clearSlot(p, "a")
	if p[0] != "" {
		t.Errorf("clearSlot did not clear slot 0: %v", p)
	}
	if playingCount(p) != 1 {
		t.Errorf("after clear, count = %d, want 1", playingCount(p))
	}
}

func TestContainsIgnoresEmpty(t *testing.T) {
	p := []string{"", "b", "", ""}
	if contains(p, "") {
		t.Error("contains should never match empty string")
	}
	if !contains(p, "b") {
		t.Error("contains should match real id")
	}
}

func TestFillFromQueue_Gathering(t *testing.T) {
	c := &model.Court{Playing: []string{"", "", "", ""}, Queue: []string{"a", "b"}}
	promoted := fillFromQueue(c)
	if !reflect.DeepEqual(c.Playing, []string{"a", "b", "", ""}) {
		t.Errorf("playing = %v", c.Playing)
	}
	if len(c.Queue) != 0 {
		t.Errorf("queue should be drained, got %v", c.Queue)
	}
	if !reflect.DeepEqual(promoted, []string{"a", "b"}) {
		t.Errorf("promoted = %v", promoted)
	}
	if c.Status != model.CourtPlaying {
		t.Errorf("status = %v, want playing", c.Status)
	}
	if c.StartedAt != "" {
		t.Errorf("started should be empty while gathering, got %q", c.StartedAt)
	}
}

func TestFillFromQueue_FillsGapsKeepingPositions(t *testing.T) {
	c := &model.Court{Playing: []string{"x", "", "", ""}, Queue: []string{"a", "b", "c", "d"}}
	promoted := fillFromQueue(c)
	if !reflect.DeepEqual(c.Playing, []string{"x", "a", "b", "c"}) {
		t.Errorf("playing = %v, want [x a b c]", c.Playing)
	}
	if !reflect.DeepEqual(c.Queue, []string{"d"}) {
		t.Errorf("queue = %v, want [d]", c.Queue)
	}
	if !reflect.DeepEqual(promoted, []string{"a", "b", "c"}) {
		t.Errorf("promoted = %v", promoted)
	}
	if c.StartedAt == "" {
		t.Error("started should be set once the court is full")
	}
}

func TestFillFromQueue_FullNoChange(t *testing.T) {
	c := &model.Court{Playing: []string{"a", "b", "c", "d"}, Queue: []string{"e"}}
	promoted := fillFromQueue(c)
	if len(promoted) != 0 {
		t.Errorf("promoted should be empty, got %v", promoted)
	}
	if !reflect.DeepEqual(c.Queue, []string{"e"}) {
		t.Errorf("queue should be untouched, got %v", c.Queue)
	}
}

func TestRecomputeStatus(t *testing.T) {
	empty := &model.Court{Playing: []string{"", "", "", ""}, StartedAt: "x"}
	recomputeStatus(empty)
	if empty.Status != model.CourtEmpty || empty.StartedAt != "" {
		t.Errorf("empty court: status=%v started=%q", empty.Status, empty.StartedAt)
	}

	gathering := &model.Court{Playing: []string{"a", "b", "", ""}, StartedAt: "x"}
	recomputeStatus(gathering)
	if gathering.Status != model.CourtPlaying || gathering.StartedAt != "" {
		t.Errorf("gathering: status=%v started=%q (started should clear < 4)", gathering.Status, gathering.StartedAt)
	}

	full := &model.Court{Playing: []string{"a", "b", "c", "d"}}
	recomputeStatus(full)
	if full.Status != model.CourtPlaying || full.StartedAt == "" {
		t.Errorf("full: status=%v started=%q (should set clock)", full.Status, full.StartedAt)
	}
}

func TestCourtNum(t *testing.T) {
	if courtNum("court-3") != 3 {
		t.Errorf("courtNum(court-3) = %d", courtNum("court-3"))
	}
	if courtNum("court#1") != 0 { // legacy '#' form no longer parsed
		t.Errorf("legacy court#1 should not parse to a number")
	}
}
