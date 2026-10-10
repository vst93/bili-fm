// Effort-slider redraws ozzy's "galaxy brain effort slider" demo
// (https://x.com/ozzyxs1a/status/2107523169649389677) in native UI: a
// card whose slider steps from Low to Max with a brain for a thumb, and
// past Max into Galaxy, where the brain bursts, the card shakes, the track
// fills with twinkling pixels and dotted rays stream out of the brain.
//
//	go run ./examples/effort-slider
//
// Drag the brain, click the track or press the arrow keys. The demo is a
// web page, so the card draws as Chrome draws it: the system font without
// font smoothing (Font.Antialiased), CSS's round corners, filter: blur()
// on the letters of the level as they change.
package main

import (
	"log"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func main() {
	s := newSlider()
	// The demo is dark whatever the system's appearance, and so are the
	// window controls over it: on Windows and Linux they draw light on
	// the dark page.
	mygo.Theme.SetSource(mygo.ThemeDark)
	mygo.App.WhenReady(func() {
		mygo.NewWindow(mygo.WindowOptions{
			Title:  "Effort",
			Width:  windowW,
			Height: windowH,
			// The card floats on the page's background, under the window
			// controls: the traffic lights on macOS, minimize, maximize and
			// close on Windows, the desktop's buttons on Linux.
			TitleBarStyle:   mygo.TitleBarHidden,
			BackgroundColor: "#101010",
			Content:         ui.View(s.view),
		})
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
