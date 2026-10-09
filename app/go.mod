module github.com/vst93/bili-fm/app

go 1.27.1

replace github.com/egoist/mygo => github.com/vst93/mygo v0.3.4-0.20261008174503-6f950516197d

require (
	github.com/ebitengine/oto/v3 v3.5.1
	github.com/egoist/mygo v0.3.4
	github.com/skip2/go-qrcode v0.0.0-20200617195104-da1b6568686e
	github.com/tphakala/go-aac v0.7.0
	github.com/tphakala/go-m4a v0.5.0
)

require (
	github.com/ebitengine/purego v0.11.1 // indirect
	github.com/go-text/typesetting v0.3.5 // indirect
	github.com/jfreymuth/pulse v0.1.3 // indirect
	github.com/tphakala/simd v1.9.0 // indirect
	golang.org/x/image v0.46.0 // indirect
	golang.org/x/sys v0.48.0 // indirect
)

tool github.com/egoist/mygo/cmd/mygo
