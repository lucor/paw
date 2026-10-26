module lucor.dev/paw

go 1.26.0

require (
	filippo.io/age v1.3.2
	fyne.io/fyne/v2 v2.8.2
	github.com/fyne-io/image v0.1.1
	github.com/godbus/dbus/v5 v5.2.2
	github.com/stretchr/testify v1.12.1
	github.com/urfave/cli/v3 v3.14.0
	go.lucor.dev/cxf v0.0.0-20261025171600-daa288fd4137
	golang.design/x/clipboard v0.11.0
	golang.org/x/crypto v0.58.0
	golang.org/x/image v0.47.0
	golang.org/x/net v0.61.0
	golang.org/x/sync v0.24.0
	golang.org/x/term v0.47.0
	gopkg.in/natefinch/npipe.v2 v2.0.0-20160621034901-c1b8fa8bdcce
)

// This replace directive is needed to allow android build
// See https://github.com/lucor/paw/pull/14
replace golang.org/x/mobile => github.com/fyne-io/gomobile-bridge v0.1.0

require (
	filippo.io/hpke v0.4.0 // indirect
	fyne.io/systray v1.12.3-0.20260810170012-af4e8e793ec4 // indirect
	fyne.io/tools v1.7.3 // indirect
	github.com/BurntSushi/toml v1.6.0 // indirect
	github.com/FyshOS/fancyfs v0.0.1 // indirect
	github.com/akavel/rsrc v0.10.2 // indirect
	github.com/anthonynsimon/bild v0.14.0 // indirect
	github.com/clipperhouse/uax29/v2 v2.2.0 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.7 // indirect
	github.com/ebitengine/purego v0.10.1 // indirect
	github.com/fogleman/gg v1.3.0 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/fyne-io/gl-js v0.2.1-0.20260315212741-029c47fd27e8 // indirect
	github.com/fyne-io/glfw-js v0.4.0 // indirect
	github.com/fyne-io/oksvg v0.2.1-0.20260918172519-a9af55fa95d9 // indirect
	github.com/go-gl/gl v0.0.0-20260331235117-4566fea9a276 // indirect
	github.com/go-gl/glfw/v3.4/glfw v0.1.0-pre.2 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/go-text/render v0.2.1 // indirect
	github.com/go-text/typesetting v0.3.5 // indirect
	github.com/golang/freetype v0.0.0-20170609003504-e2365dfdc4a0 // indirect
	github.com/hack-pad/go-indexeddb v0.3.2 // indirect
	github.com/hack-pad/safejs v0.1.0 // indirect
	github.com/jackmordaunt/icns/v2 v2.2.7 // indirect
	github.com/jeandeaual/go-locale v0.0.0-20250612000132-0ef82f21eade // indirect
	github.com/josephspurrier/goversioninfo v1.7.0 // indirect
	github.com/jsummers/gobmp v0.0.0-20230614200233-a9de23ed2e25 // indirect
	github.com/lucor/goinfo v0.9.0 // indirect
	github.com/mattn/go-runewidth v0.0.24 // indirect
	github.com/mcuadros/go-version v0.0.0-20190830083331-035f6764e8d2 // indirect
	github.com/natefinch/atomic v1.0.1 // indirect
	github.com/nfnt/resize v0.0.0-20180221191011-83c6a9932646 // indirect
	github.com/nicksnyder/go-i18n/v2 v2.5.1 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/rymdport/portal v0.4.2 // indirect
	github.com/srwiley/oksvg v0.0.0-20221011165216-be6e8873101c // indirect
	github.com/srwiley/rasterx v0.0.0-20220730225603-2ab79fcdd4ef // indirect
	github.com/urfave/cli/v2 v2.27.7 // indirect
	github.com/xrash/smetrics v0.0.0-20240521201337-686a1a2994c1 // indirect
	github.com/yuin/goldmark v1.8.2 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.design/x/x11 v0.2.0 // indirect
	golang.org/x/exp/typeparams v0.0.0-20231108232855-2478ac86f678 // indirect
	golang.org/x/mobile v0.0.0-20250606033058-a2a15c67f36f // indirect
	golang.org/x/mod v0.41.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
	golang.org/x/telemetry v0.0.0-20260924152758-ed294f943157 // indirect
	golang.org/x/text v0.43.0 // indirect
	golang.org/x/tools v0.51.0 // indirect
	golang.org/x/tools/go/vcs v0.1.0-deprecated // indirect
	golang.org/x/vuln v1.8.0 // indirect
	honnef.co/go/tools v0.8.2 // indirect
)

tool (
	fyne.io/tools/cmd/fyne
	golang.org/x/vuln/cmd/govulncheck
	honnef.co/go/tools/cmd/staticcheck
)
