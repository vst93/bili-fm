# Fetch

The fetch plugin gives pages a `fetch` that makes HTTP requests from Go. It
takes the options of the standard `fetch` and returns a standard
`Response`, but the request leaves from the Go side of the app, so:

- it is not subject to CORS: any server answers, and every response header
  is readable but `Set-Cookie`, which a `Response` cannot hold;
- it may set any header, including those the webview forbids, such as
  `Origin`, `Cookie`, `Referer` and `User-Agent`;
- its body streams to the page as Go reads it, and aborting it cancels the
  request in Go.

## Set up

Use the plugin in Go:

```go
import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/fetch"
)

func main() {
	mygo.Use(fetch.Plugin)
	// ...
}
```

and add its package to the frontend:

```sh
bun add @mygo-plugins/fetch
```

Calling its `fetch` in an app that does not use the Go half rejects with an
error that says to call `mygo.Use`.

## Making requests

Import `fetch` from the package and call it as you would the webview's:

```ts
import { fetch } from "@mygo-plugins/fetch";

const res = await fetch("https://api.example.com/items", {
  method: "POST",
  headers: {
    authorization: `Bearer ${token}`,
    "content-type": "application/json",
  },
  body: JSON.stringify({ name: "Ada" }),
});
if (!res.ok) throw new Error(`HTTP ${res.status}`);
const item = await res.json();
```

A `Request` works in place of the URL. Bodies are those of the standard
`fetch`, such as strings, `FormData`, `URLSearchParams`, `Blob`s and
`ArrayBuffer`s, with the `Content-Type` it would give them; they are read
whole before the request starts, streams too.

Libraries that take a `fetch` of their own, as many API clients do, can be
given this one. Do not assign it to `globalThis.fetch`: the plugin hands
the URLs it does not make, those whose scheme is not http or https, to the
webview's `fetch`.

## Streaming responses

The body of the response is a `ReadableStream` that Go fills as the server
sends it, which suits server-sent events and the streamed answers of model
APIs:

```ts
const res = await fetch("https://api.example.com/events");
const reader = res.body!.pipeThrough(new TextDecoderStream()).getReader();
for (;;) {
  const { value, done } = await reader.read();
  if (done) break;
  output.append(value);
}
```

Go reads no faster than the page: it waits while the page has more than a
MiB of the body yet to read, so a large download does not pile up in the
page's memory.

## Canceling

Aborting the request's `signal` cancels it in Go, before or after the
response arrived:

```ts
const ctrl = new AbortController();
const timer = setTimeout(() => ctrl.abort(), 10_000);
try {
  const res = await fetch(url, { signal: ctrl.signal });
  return await res.text();
} finally {
  clearTimeout(timer);
}
```

The promise, or the reading of the body, rejects with the signal's reason.
Canceling the body (`res.body.cancel()`), navigating away and closing the
window cancel the request too.

## Errors

As with the webview's `fetch`, a response is not an error, whatever its
status: check `res.ok`. The promise rejects with a `TypeError` when the
request fails, such as when the server cannot be reached, TLS fails or
there are too many redirects, and its message says why:

```ts
try {
  await fetch("https://unreachable.example");
} catch (err) {
  console.log(err.message); // fetch failed: Get "https://unreachable.example": dial tcp: lookup unreachable.example: no such host
}
```

## Differences from the webview's fetch

- `mode`, `credentials`, `cache`, `referrerPolicy`, `integrity`,
  `keepalive` and `priority` do not apply and are ignored.
- Cookies are those of the Go side's `http.Client`, never the webview's:
  none, unless the client has a `Jar` to keep them (see
  [options](#options)).
- `redirect: "manual"` returns the redirect response itself, with its
  status and `Location`, instead of an opaque response. `"error"` rejects
  on a redirect, and `"follow"`, the default, follows up to 20 redirects
  and sets `res.redirected` and `res.url`.
- URLs whose scheme is not http or https, such as the app's own pages in
  builds (`mygo://localhost`), `data:` and `blob:` URLs, go to the
  webview's `fetch`. Relative URLs resolve against the page's: during
  `mygo dev` they are those of the dev server, which Go fetches.

## Options

`fetch.Plugin` makes requests with `http.DefaultClient` and lets pages
request any URL. `fetch.New` takes options instead:

```go
jar, _ := cookiejar.New(nil)

mygo.Use(fetch.New(fetch.Options{
	Client: &http.Client{Jar: jar, Timeout: time.Minute},
	Allow: func(r *http.Request) bool {
		return r.URL.Hostname() == "api.example.com"
	},
}))
```

- `Client` makes the requests, so its `Transport` (proxies, TLS settings),
  `Jar` (cookies kept between requests) and `Timeout` apply, and its
  `CheckRedirect` limits the redirects followed. `http.DefaultClient`
  takes its proxy from the environment (`HTTPS_PROXY`, `NO_PROXY`), not
  from the system's settings.
- `Allow` sees each request before Go makes it, with its URL, method and
  headers, and decides whether pages may make it: a request it refuses
  rejects with an error. It sees the first request only, not the
  redirects that follow, which the client's `CheckRedirect` can refuse.

Only the app's own pages may call the plugin, as with every bound method
(see [who may call](../bindings.md#who-may-call)); `Allow` keeps a page
that runs code it should not, such as through a cross-site scripting bug,
from reaching other servers.
