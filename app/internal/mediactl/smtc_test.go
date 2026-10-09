package mediactl

import "testing"

// smtcState 是 Windows SMTC 里一帧要推的状态（抽成纯结构，方便单测）。
type smtcState struct {
	Status     Status
	Title      string
	Artist     string
	Album      string
	LengthUs   int64
	PositionUs int64
	Volume     float64
	Rate       float64
	Next       bool
	Prev       bool
	Loop       string
	Shuffle    bool
}

// equals 判断两个状态帧是否等价（位置差 100ms 以内视为相同）。
func (s smtcState) same(o smtcState) bool {
	posDelta := s.PositionUs - o.PositionUs
	if posDelta < 0 {
		posDelta = -posDelta
	}
	return s.Status == o.Status && s.Title == o.Title && s.Artist == o.Artist &&
		s.Album == o.Album && s.LengthUs == o.LengthUs &&
		posDelta <= 100_000 && s.Volume == o.Volume && s.Rate == o.Rate &&
		s.Next == o.Next && s.Prev == o.Prev && s.Loop == o.Loop && s.Shuffle == o.Shuffle
}

func TestSmtcStateSame(t *testing.T) {
	base := smtcState{Status: Playing, Title: "a", PositionUs: 1000}
	if !base.same(smtcState{Status: Playing, Title: "a", PositionUs: 1000 + 50_000}) {
		t.Error("位置差 50ms 应视为相同")
	}
	if base.same(smtcState{Status: Paused, Title: "a", PositionUs: 1000}) {
		t.Error("状态不同应视为不同")
	}
	if base.same(smtcState{Status: Playing, Title: "b", PositionUs: 1000}) {
		t.Error("标题不同应视为不同")
	}
}
