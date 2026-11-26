package main

import (
	"fmt"
	"testing"

	"github.com/vst93/bili-fm/app/internal/view"
)

func TestSectionCardsRetainCompleteWatchLater(t *testing.T) {
	cards := make([]view.Card, maxRetainedCards+40)
	for i := range cards {
		cards[i].Bvid = fmt.Sprintf("BV%04d", i)
	}
	for _, tc := range []struct {
		section, tab string
		complete     bool
	}{
		{"history", view.HistWatchLater, true},
		{"watchlater", "", true},
		{"history", view.HistHistory, false},
		{"feed", "", false},
		{"popular", "", false},
	} {
		t.Run(tc.section+tc.tab, func(t *testing.T) {
			list := &view.List{Cards: []view.Card{{Bvid: "old"}}, Dropped: true, Trimmed: 6}
			added := setSectionCards(list, append(cards, cards[0]), true, tc.section, tc.tab)
			if added != len(cards) {
				t.Fatalf("新增数（去重后、裁头前）=%d, want %d", added, len(cards))
			}
			if tc.complete {
				if len(list.Cards) != len(cards) || list.Cards[0].Bvid != cards[0].Bvid || list.Dropped || list.Trimmed != 0 {
					t.Fatalf("稍后再看丢失内容：cards=%d first=%s dropped=%v trimmed=%d", len(list.Cards), list.Cards[0].Bvid, list.Dropped, list.Trimmed)
				}
			} else if len(list.Cards) > maxRetainedCards || !list.Dropped {
				t.Fatalf("分页列表不应取消内存上限：cards=%d dropped=%v", len(list.Cards), list.Dropped)
			}
		})
	}
}
