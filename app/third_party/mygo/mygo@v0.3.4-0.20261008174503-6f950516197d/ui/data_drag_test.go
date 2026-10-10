package ui

import (
	"errors"
	"slices"
	"testing"

	"github.com/egoist/mygo/internal/platform"
	"github.com/egoist/mygo/transfer"
)

func beginNativeDrag(tt *Tester) {
	tt.Press(10, 10)
	tt.Move(20, 20)
}

func TestNativeDataDropNegotiatesAndReadsOnDrop(t *testing.T) {
	var got []transfer.Drop
	var results []transfer.Result
	var over bool
	calls, clicks := 0, 0
	custom := transfer.Format("application/vnd.mygo.test+json")
	data := transfer.New(transfer.NewItem(transfer.Bytes(transfer.Text, []byte("fallback")), transfer.Lazy(custom, func() ([]byte, error) { calls++; return []byte(`{"id":7}`), nil })))
	source := coreNewTester(func(c *context) {
		s := coreBox(c).Size(100, 50).DragData(data, transfer.DragOptions{Operations: transfer.Copy | transfer.Move, Done: func(r transfer.Result) { results = append(results, r) }})
		if s.Clicked() {
			clicks++
		}
	}, 200, 200)
	target := coreNewTester(func(c *context) {
		zone := coreBox(c).Size(100, 100)
		opts := transfer.DropOptions{Formats: []transfer.Format{custom}, Operations: transfer.Copy | transfer.Move}
		_, over = coreDataDragOver(zone, opts)
		if d, ok := coreDropData(zone, opts); ok {
			got = append(got, d)
		}
	}, 200, 200)
	beginNativeDrag(source)
	if calls != 0 || source.h.dragOptions.Preview == nil {
		t.Fatal("source eagerly read data or omitted its default preview")
	}
	d := &platform.DataDragEvent{Offer: transfer.Offer{Formats: data.Formats(), Operations: transfer.Copy | transfer.Move, Suggested: transfer.Move}}
	if !target.send2(platform.SurfaceEvent{Kind: platform.DataDragOver, X: 10, Y: 10, Drag: d}) || !over || calls != 0 || d.Operation != transfer.Move {
		t.Fatalf("hover %v, calls %d, effect %v", over, calls, d.Operation)
	}
	d.Data = source.h.dragData
	if !target.send2(platform.SurfaceEvent{Kind: platform.DataDrop, X: 10, Y: 10, Drag: d}) {
		t.Fatal("drop was rejected")
	}
	source.h.finishDataDrag(transfer.Result{Operation: d.Operation})
	source.settle()
	if len(got) != 1 || got[0].Operation != transfer.Move || calls != 1 || over || clicks != 0 || len(results) != 1 {
		t.Fatalf("drops %v, calls %d, results %v, clicks %d, over %v", got, calls, results, clicks, over)
	}
	if !slices.Equal(got[0].Data.Formats(), []transfer.Format{custom}) {
		t.Fatal("unrequested fallback was read")
	}
	target.Frame()
	if len(got) != 1 {
		t.Fatal("drop delivered twice")
	}
}

func TestNativeTypedDropKeepsGoValue(t *testing.T) {
	value := &struct{ Name string }{"same object"}
	var got any
	var over bool
	source := coreNewTester(func(c *context) { coreBox(c).Size(80, 40).Drag(value).DragData(transfer.TextData(value.Name)) }, 200, 200)
	target := coreNewTester(func(c *context) {
		e := coreBox(c).Size(100, 100)
		_, over = coreDragOver[*struct{ Name string }](e)
		if v, ok := coreDrop[*struct{ Name string }](e); ok {
			got = v
		}
	}, 200, 200)
	beginNativeDrag(source)
	d := &platform.DataDragEvent{Offer: transfer.Offer{Formats: []transfer.Format{transfer.Text}, Operations: transfer.Copy}, Local: source.h.dragLocal}
	if !target.send2(platform.SurfaceEvent{Kind: platform.DataDragOver, X: 5, Y: 5, Drag: d}) || !over {
		t.Fatal("typed hover failed")
	}
	if !target.send2(platform.SurfaceEvent{Kind: platform.DataDrop, X: 5, Y: 5, Drag: d}) || got != value {
		t.Fatal("typed identity was lost")
	}
	source.h.finishDataDrag(transfer.Result{Operation: d.Operation})
}

func TestNativeDragCancellationAndSourceRemoval(t *testing.T) {
	for _, remove := range []bool{false, true} {
		t.Run(map[bool]string{false: "escape", true: "source removal"}[remove], func(t *testing.T) {
			show := true
			var results []transfer.Result
			tt := coreNewTester(func(c *context) {
				if show {
					coreBox(c).Size(80, 40).DragData(transfer.TextData("a"), transfer.DragOptions{Done: func(r transfer.Result) { results = append(results, r) }})
				}
			}, 200, 200)
			beginNativeDrag(tt)
			if remove {
				show = false
				tt.Frame()
			} else {
				tt.Key(0, KeyEscape)
			}
			tt.Release(80, 80)
			tt.h.finishDataDrag(transfer.Result{Operation: transfer.Copy})
			if len(results) != 1 || !results[0].Canceled || tt.rt.drag != nil || tt.h.dragLocal != nil {
				t.Fatalf("results %v, drag %v", results, tt.rt.drag)
			}
		})
	}
}

func TestNativeDragCloseReleasesTextInputClient(t *testing.T) {
	client := &primitiveClient{text: "draft", mark: &TextInputRange{Start: 0, End: 5}}
	var results []transfer.Result
	var tt *Tester
	tt = coreNewTester(func(c *context) {
		coreBox(c).Size(80, 40).HandleTextInput(client).AutoFocus().DragData(transfer.TextData("draft"), transfer.DragOptions{
			Done: func(r transfer.Result) {
				results = append(results, r)
				tt.h.ime.Client.ReplaceText(nil, "stale completion")
			},
		})
	}, 200, 200)
	beginNativeDrag(tt)
	if tt.rt.drag == nil || !tt.rt.drag.native {
		t.Fatal("native drag did not start")
	}
	nativeClient := tt.h.ime.Client
	tt.rt.close()
	if len(results) != 1 || !results[0].Canceled || tt.rt.drag != nil || tt.h.dragOptions.Done != nil {
		t.Fatalf("close did not cancel the drag: results %v, drag %v", results, tt.rt.drag)
	}
	if client.mark != nil || client.unmarks != 1 {
		t.Fatal("close did not release composition", client.mark, client.unmarks)
	}
	nativeClient.ReplaceText(nil, "stale native callback")
	if client.changes != 0 || client.text != "draft" {
		t.Fatal("closed text-input client mutated", client.text, client.changes)
	}
	tt.rt.close()
	if len(results) != 1 || client.unmarks != 1 {
		t.Fatal("close repeated cleanup", results, client.unmarks)
	}
}

func TestNativeDestinationRejectsMismatchDisabledAndFailedData(t *testing.T) {
	disabled, drops := false, 0
	tt := coreNewTester(func(c *context) {
		e := coreBox(c).Size(100, 100).Disabled(disabled)
		if _, ok := coreDropData(e, transfer.DropOptions{Formats: []transfer.Format{transfer.Text}, Operations: transfer.Copy}); ok {
			drops++
		}
	}, 200, 200)
	for _, offer := range []transfer.Offer{{Formats: []transfer.Format{transfer.PNG}, Operations: transfer.Copy}, {Formats: []transfer.Format{transfer.Text}, Operations: transfer.Move}} {
		d := &platform.DataDragEvent{Offer: offer}
		if tt.send2(platform.SurfaceEvent{Kind: platform.DataDragOver, X: 10, Y: 10, Drag: d}) || d.Operation != transfer.None {
			t.Fatal("incompatible transfer accepted")
		}
	}
	disabled = true
	tt.Frame()
	d := &platform.DataDragEvent{Offer: transfer.Offer{Formats: []transfer.Format{transfer.Text}, Operations: transfer.Copy}}
	if tt.send2(platform.SurfaceEvent{Kind: platform.DataDragOver, X: 10, Y: 10, Drag: d}) {
		t.Fatal("disabled destination accepted")
	}
	disabled = false
	tt.Frame()
	d.Data = transfer.New(transfer.NewItem(transfer.Lazy(transfer.Text, func() ([]byte, error) { return nil, errors.New("failed") })))
	if tt.send2(platform.SurfaceEvent{Kind: platform.DataDrop, X: 10, Y: 10, Drag: d}) || d.Operation != transfer.None || drops != 0 {
		t.Fatal("failed provider acknowledged a drop")
	}
}
