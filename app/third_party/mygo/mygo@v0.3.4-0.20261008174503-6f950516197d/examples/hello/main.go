// Hello is the smallest MyGo app: one window and one Go method called from
// the page.
//
//	go run ./examples/hello
package main

import (
	"log"
	"runtime"

	"github.com/egoist/mygo"
)

// Greeter is callable from the page. In a real project `mygo generate`
// writes a typed TypeScript client for it; this example uses the untyped
// window.mygo.call to stay in a single file.
type Greeter struct{}

// Greet returns a greeting.
func (Greeter) Greet(name string) string {
	return "Hello " + name + ", from Go " + runtime.Version() + "!"
}

const page = `<!doctype html>
<html>
<head>
  <title>Hello MyGo</title>
  <style>
    :root { color-scheme: light dark; font-family: system-ui, sans-serif; }
    body { height: 100vh; margin: 0; display: grid; place-content: center; text-align: center; }
  </style>
</head>
<body>
  <h1>Hello MyGo</h1>
  <p id="greeting">…</p>
  <script>
    mygo.call("Greeter.Greet", "world").then((text) => {
      document.getElementById("greeting").textContent = text;
    });
  </script>
</body>
</html>`

func main() {
	mygo.Bind(Greeter{})
	mygo.App.WhenReady(func() {
		win := mygo.NewWindow(mygo.WindowOptions{Width: 480, Height: 320})
		win.Page().LoadHTML(page, "")
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
