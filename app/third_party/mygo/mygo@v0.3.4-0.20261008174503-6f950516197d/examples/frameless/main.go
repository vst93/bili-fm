// Frameless shows a window without native chrome: the page draws its own
// title bar, marks it draggable with `--app-region: drag` and drives the
// window through the built-in window.mygo.window controls.
//
//	go run ./examples/frameless
package main

import (
	"log"
	"runtime"

	"github.com/egoist/mygo"
)

const page = `<!doctype html>
<html>
<head>
  <title>Frameless</title>
  <style>
    :root { color-scheme: light dark; font-family: system-ui, sans-serif; }
    html, body { height: 100%; margin: 0; }
    body { display: flex; flex-direction: column; background: rgba(128, 128, 128, 0.08); border-radius: 12px; }
    .titlebar {
      --app-region: drag;           /* drag the window by the bar */
      height: 44px; display: flex; align-items: center; gap: 8px; padding: 0 12px;
      background: linear-gradient(90deg, #6366f1, #a855f7); color: white; user-select: none;
    }
    .titlebar .title { flex: 1; font-weight: 600; }
    .titlebar button {             /* buttons stay clickable */
      width: 28px; height: 28px; border: 0; border-radius: 6px;
      background: rgba(255, 255, 255, 0.2); color: white; font-size: 14px; cursor: pointer;
    }
    .titlebar button:hover { background: rgba(255, 255, 255, 0.35); }
    main { flex: 1; display: grid; place-content: center; text-align: center; padding: 24px; }
  </style>
</head>
<body>
  <div class="titlebar">
    <span class="title">Custom title bar</span>
    <button id="min" title="Minimize">–</button>
    <button id="max" title="Maximize">□</button>
    <button id="close" title="Close">✕</button>
  </div>
  <main>
    <h2>No native frame</h2>
    <p>Drag the gradient bar to move the window.<br>Double-click it to zoom.</p>
  </main>
  <script>
    const w = mygo.window;
    document.getElementById("min").onclick = () => w.minimize();
    document.getElementById("max").onclick = () => w.toggleMaximize();
    document.getElementById("close").onclick = () => w.close();
  </script>
</body>
</html>`

func main() {
	mygo.App.WhenReady(func() {
		opts := mygo.WindowOptions{
			Width:       640,
			Height:      420,
			MinWidth:    360,
			MinHeight:   240,
			Frameless:   true,
			Transparent: true,
		}
		if runtime.GOOS == "darwin" {
			// A translucent material behind the transparent page.
			opts.Vibrancy = mygo.VibrancyUnderWindow
		}
		mygo.NewWindow(opts).Page().LoadHTML(page, "")
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
