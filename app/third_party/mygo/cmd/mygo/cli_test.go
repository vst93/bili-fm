package main

import (
	"bytes"
	"debug/pe"
	"encoding/binary"
	"encoding/json"
	"go/parser"
	"go/token"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestConfigDefaults(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "My App")
	if err := os.MkdirAll(filepath.Join(dir, "frontend"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "frontend", "package.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := loadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if c.Name != "My App" || c.Identifier != "com.mygo.myapp" || c.Version != "0.1.0" || c.Main != "." {
		t.Errorf("unexpected defaults: %+v", c)
	}
	if c.DevURL != "" || c.DevCommand != "" || c.BuildCommand != "" || c.FrontendDist != "" {
		t.Errorf("frontend options are opt-in: %+v", c)
	}
	if c.Bindings != filepath.Join("frontend", "src", "mygo.ts") {
		t.Errorf("bindings = %q", c.Bindings)
	}

	write := func(json string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, "mygo.json"), []byte(json), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(`{"name":"Custom","bindings":"web/api.ts","devUrl":"http://localhost:3000","devCommand":"bun run dev","buildCommand":"bun run build","frontendDist":"web/dist"}`)
	c, err = loadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if c.Name != "Custom" || c.Bindings != "web/api.ts" || c.DevURL != "http://localhost:3000" || c.DevCommand != "bun run dev" || c.BuildCommand != "bun run build" || c.FrontendDist != "web/dist" {
		t.Errorf("mygo.json not applied: %+v", c)
	}
	write(`{"devUrl":"localhost:3000"}`)
	if _, err := loadConfig(dir); err == nil || !strings.Contains(err.Error(), "devUrl") {
		t.Errorf("invalid devUrl accepted: %v", err)
	}

	// A frontend at the project root.
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if c, err := loadConfig(root); err != nil || c.Bindings != filepath.Join("src", "mygo.ts") {
		t.Errorf("bindings with package.json at the root = %q, %v", c.Bindings, err)
	}
}

func TestInfoPlist(t *testing.T) {
	c := &Config{Name: "A & B", Identifier: "com.example.ab", Version: "1.2.3", MacOS: MacOS{MinimumSystemVersion: "12.0"}, URLSchemes: []string{"ab"}}
	plist := string(infoPlist(c, "A & B", "icon.icns"))
	for _, want := range []string{
		"<string>A &amp; B</string>",
		"<key>CFBundleIdentifier</key>\n\t<string>com.example.ab</string>",
		"<key>CFBundleIconFile</key>\n\t<string>icon.icns</string>",
		"<key>CFBundleURLSchemes</key>",
		"<string>ab</string>",
	} {
		if !strings.Contains(plist, want) {
			t.Errorf("Info.plist missing %q:\n%s", want, plist)
		}
	}
}

func TestIcons(t *testing.T) {
	src := defaultIcon()
	if err := validateIcon(src); err != nil {
		t.Fatal(err)
	}
	icns, err := pngToICNS(src)
	if err != nil {
		t.Fatal(err)
	}
	if string(icns[:4]) != "icns" || int(binary.BigEndian.Uint32(icns[4:])) != len(icns) {
		t.Fatalf("bad icns header")
	}
	// Walk the entries and decode one of them.
	found := 0
	for off := 8; off < len(icns); {
		kind, size := string(icns[off:off+4]), int(binary.BigEndian.Uint32(icns[off+4:]))
		if kind == "ic07" {
			img, err := png.Decode(bytes.NewReader(icns[off+8 : off+size]))
			if err != nil || img.Bounds().Dx() != 128 {
				t.Errorf("ic07 entry: %v", err)
			}
		}
		off += size
		found++
	}
	if found != 10 {
		t.Errorf("icns has %d entries", found)
	}
	ico, err := pngToICO(src)
	if err != nil {
		t.Fatal(err)
	}
	if binary.LittleEndian.Uint16(ico[2:]) != 1 || binary.LittleEndian.Uint16(ico[4:]) != 7 {
		t.Error("bad ico header")
	}
}

func TestWriteUniversal(t *testing.T) {
	dir := t.TempDir()
	machO := func(name string, cpu uint32, size int) string {
		b := make([]byte, size)
		binary.LittleEndian.PutUint32(b, 0xfeedfacf)
		binary.LittleEndian.PutUint32(b[4:], cpu)
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, b, 0o755); err != nil {
			t.Fatal(err)
		}
		return p
	}
	arm, amd := machO("arm", 0x0100000c, 1000), machO("amd", 0x01000007, 3000)
	out := filepath.Join(dir, "fat")
	if err := writeUniversal(out, arm, amd); err != nil {
		t.Fatal(err)
	}
	fat, _ := os.ReadFile(out)
	if binary.BigEndian.Uint32(fat) != 0xcafebabe || binary.BigEndian.Uint32(fat[4:]) != 2 {
		t.Fatal("bad fat header")
	}
	for i, wantSize := range []uint32{1000, 3000} {
		entry := fat[8+i*20:]
		off, size := binary.BigEndian.Uint32(entry[8:]), binary.BigEndian.Uint32(entry[12:])
		if size != wantSize || off%(1<<14) != 0 || binary.LittleEndian.Uint32(fat[off:]) != 0xfeedfacf {
			t.Errorf("slice %d: offset %d size %d", i, off, size)
		}
	}
	if err := writeUniversal(out, filepath.Join(dir, "missing")); err == nil {
		t.Error("expected error for missing slice")
	}
}

func TestTemplate(t *testing.T) {
	dir := t.TempDir()
	data := templateData{Name: `The "Demo" App`, Slug: "demo-app", Module: "demo-app", Identifier: "com.example.demoapp",
		Runtime: "^0.1.0", CLI: "^0.1.0", Mygo: "mygo", ConfigImport: "mygo-cli"}
	if err := writeTemplate(dir, webTemplate, data); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"main.go", tsConfig, ".gitignore", "package.json", "vite.config.ts", "index.html", "tsconfig.json", "src/main.ts", "src/style.css"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Errorf("missing %s", f)
		}
	}
	main, _ := os.ReadFile(filepath.Join(dir, "main.go"))
	if !strings.Contains(string(main), `Title:           "The \"Demo\" App"`) {
		t.Errorf("name not rendered into main.go:\n%s", main)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "main.go", main, 0); err != nil {
		t.Errorf("main.go does not parse: %v", err)
	}
	if html, _ := os.ReadFile(filepath.Join(dir, "index.html")); !strings.Contains(string(html), "<title>The &#34;Demo&#34; App</title>") {
		t.Errorf("index.html:\n%s", html)
	}
	var pkg struct {
		Name            string
		Scripts         map[string]string
		Dependencies    map[string]string
		DevDependencies map[string]string
	}
	raw, _ := os.ReadFile(filepath.Join(dir, "package.json"))
	if err := json.Unmarshal(raw, &pkg); err != nil {
		t.Fatalf("package.json: %v\n%s", err, raw)
	}
	if pkg.Name != "demo-app" || pkg.Scripts["dev"] != "mygo dev" || pkg.Dependencies["mygo-runtime"] != "^0.1.0" || pkg.DevDependencies["mygo-cli"] != "^0.1.0" {
		t.Errorf("package.json: %s", raw)
	}

	// A project using a checkout of MyGo runs its CLI with go run, and
	// imports defineConfig from the checkout.
	local := t.TempDir()
	cli, _ := filepath.Abs(filepath.Join("..", "..", "packages", "cli", "index.js"))
	local2 := data
	local2.CLI, local2.Mygo, local2.ConfigImport = "", "go run github.com/egoist/mygo/cmd/mygo", moduleSpecifier(local, cli)
	if err := writeTemplate(local, webTemplate, local2); err != nil {
		t.Fatal(err)
	}
	raw, _ = os.ReadFile(filepath.Join(local, "package.json"))
	pkg.DevDependencies = nil
	if err := json.Unmarshal(raw, &pkg); err != nil || pkg.DevDependencies["mygo-cli"] != "" || pkg.Scripts["build"] != local2.Mygo+" build" {
		t.Errorf("package.json of a local checkout (%v): %s", err, raw)
	}

	if _, err := configRuntime(); err != nil {
		t.Skipf("reading mygo.config.ts: %v", err)
	}
	// In a project, mygo-cli is installed.
	os.Mkdir(filepath.Join(dir, "node_modules"), 0o755)
	if err := os.Symlink(filepath.Dir(cli), filepath.Join(dir, "node_modules", "mygo-cli")); err != nil {
		t.Logf("not reading the project's configuration: %v", err)
		dir = ""
	}
	for _, root := range []string{dir, local} {
		if root == "" {
			continue
		}
		c, err := loadConfig(root)
		if err != nil {
			t.Fatal(err)
		}
		if c.Name != data.Name || c.DevURL != "http://localhost:5173" || c.FrontendDist != "dist" || c.Out != "build" || c.Bindings != "src/mygo.ts" {
			t.Errorf("template configuration: %+v", c)
		}
		// devCommand and buildCommand run scripts of package.json, which
		// must not run mygo again.
		for _, cmd := range []string{c.DevCommand, c.BuildCommand} {
			script, ok := strings.CutPrefix(cmd, "bun run ")
			if !ok || pkg.Scripts[script] == "" || strings.Contains(pkg.Scripts[script], "mygo") {
				t.Errorf("%q does not run a frontend script of package.json", cmd)
			}
		}
	}
	if slugify("Hello, World!") != "hello-world" || slugify("!!!") != "app" {
		t.Error("slugify")
	}
}

func TestTemplateSkills(t *testing.T) {
	for _, tmpl := range []string{webTemplate, nativeTemplate} {
		t.Run(tmpl, func(t *testing.T) {
			dir := t.TempDir()
			if err := writeTemplate(dir, tmpl, templateData{Name: "Demo", Slug: "demo"}); err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"SKILL.md", "agents/openai.yaml"} {
				path := ".agents/skills/mygo-maintenance/" + name
				want, err := templateFS.ReadFile("template/shared/" + path)
				if err != nil {
					t.Fatal(err)
				}
				got, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(path)))
				if err != nil {
					t.Fatal(err)
				}
				// In particular, the skill's {{.Dir}} example is literal
				// Markdown, not a Go template to evaluate.
				if !bytes.Equal(got, want) {
					t.Errorf("%s: shared skill content changed during scaffolding", path)
				}
			}
		})
	}
}

// TestNativeTemplate writes the template of native UI, which has no
// frontend, and builds its app and runs its test against this checkout.
func TestNativeTemplate(t *testing.T) {
	dir := t.TempDir()
	data := templateData{Name: `The "Demo" App`, Slug: "demo-app", Module: "demo-app", Identifier: "com.example.demoapp"}
	if err := writeTemplate(dir, nativeTemplate, data); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"main.go", "main_test.go", jsonConfig, ".gitignore"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Errorf("missing %s", f)
		}
	}
	for _, f := range []string{"package.json", "index.html", "src"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err == nil {
			t.Errorf("the native template has %s", f)
		}
	}
	main, _ := os.ReadFile(filepath.Join(dir, "main.go"))
	if !strings.Contains(string(main), `Title:     "The \"Demo\" App"`) {
		t.Errorf("name not rendered into main.go:\n%s", main)
	}
	c, err := loadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if c.Name != data.Name || c.Identifier != data.Identifier || c.Out != "build" || c.Bindings != "" || c.DevURL != "" || c.FrontendDist != "" {
		t.Errorf("template configuration: %+v", c)
	}

	if testing.Short() {
		t.Skip("building the app")
	}
	checkout, _ := filepath.Abs(filepath.Join("..", ".."))
	sum, _ := os.ReadFile(filepath.Join(checkout, "go.sum"))
	gomod := "module demo-app\n\ngo " + goVersion() + "\n\nrequire github.com/egoist/mygo v0.0.0\n\nreplace github.com/egoist/mygo => " + checkout + "\n"
	if os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0o644) != nil || os.WriteFile(filepath.Join(dir, "go.sum"), sum, 0o644) != nil {
		t.Fatal("cannot write the module")
	}
	// The modules MyGo needs are in the cache: nothing is downloaded.
	for _, args := range [][]string{{"vet", "."}, {"test", "-count=1", "."}} {
		cmd := goCommand(dir, []string{"GOFLAGS=-mod=mod", "GOPROXY=off", "GOWORK=off"}, args...)
		var out bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &out
		if err := cmd.Run(); err != nil {
			t.Fatalf("go %s in the native template: %v\n%s", strings.Join(args, " "), err, out.String())
		}
	}
}

func TestModuleSpecifier(t *testing.T) {
	root := filepath.FromSlash("/work")
	for target, want := range map[string]string{
		"/work/mygo/packages/cli/index.js": "./mygo/packages/cli/index.js",
		"/src/mygo/packages/cli/index.js":  "../src/mygo/packages/cli/index.js",
	} {
		if got := moduleSpecifier(root, filepath.FromSlash(target)); got != want {
			t.Errorf("moduleSpecifier(%s) = %s, want %s", target, got, want)
		}
	}
}

func TestPackageFlags(t *testing.T) {
	c := &Config{Name: "Bob's App", Version: "1.0.0", Identifier: "com.example.bob", URLSchemes: []string{"bob", "bob-dev"}}
	got := packageFlags(c)
	want := ` -X "github.com/egoist/mygo.packageName=Bob's App" -X github.com/egoist/mygo.packageVersion=1.0.0` +
		` -X github.com/egoist/mygo.packageIdentifier=com.example.bob -X github.com/egoist/mygo.packageURLSchemes=bob,bob-dev`
	if got != want {
		t.Errorf("packageFlags =\n%s\nwant\n%s", got, want)
	}
	if q := ldflagsQuote(`say "hi"`); q != `'say "hi"'` {
		t.Errorf("ldflagsQuote = %s", q)
	}

	dir := t.TempDir()
	write := func(json string) error {
		if err := os.WriteFile(filepath.Join(dir, "mygo.json"), []byte(json), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := loadConfig(dir)
		return err
	}
	if err := write(`{"urlSchemes": ["my app"]}`); err == nil || !strings.Contains(err.Error(), "urlSchemes") {
		t.Errorf("invalid scheme: %v", err)
	}
	if err := write(`{"name": "Both ' and \""}`); err == nil || !strings.Contains(err.Error(), "quotes") {
		t.Errorf("name with both quotes: %v", err)
	}

	// The desktop entry of Linux builds opens the app's URLs.
	c = &Config{Name: "My App", URLSchemes: []string{"myapp"}}
	files, err := writeLinuxDesktop(c, dir, "my-app")
	if err != nil {
		t.Fatal(err)
	}
	entry, _ := os.ReadFile(files[len(files)-1])
	for _, want := range []string{"Exec=my-app %U\n", "MimeType=x-scheme-handler/myapp;\n"} {
		if !strings.Contains(string(entry), want) {
			t.Errorf("desktop entry lacks %q:\n%s", want, entry)
		}
	}
}

// TestWindowsSignCommand signs a file whose path the shell gets quoted.
func TestProductionTags(t *testing.T) {
	t.Setenv("GOFLAGS", "")
	t.Setenv("MYGO_INSPECTOR", "")
	tags := func(debug bool) string { return strings.Join(productionTags(debug), " ") }
	if got := tags(false); got != "-tags mygo_noinspector" {
		t.Errorf("a production build has %q", got)
	}
	if got := tags(true); got != "" {
		t.Errorf("a debug build has %q", got)
	}
	// The tags of GOFLAGS stay: -tags would override them.
	t.Setenv("GOFLAGS", "-mod=mod -tags=sqlite,fts5")
	if got := tags(false); got != "-tags sqlite,fts5,mygo_noinspector" {
		t.Errorf("with GOFLAGS' tags, a production build has %q", got)
	}
	t.Setenv("MYGO_INSPECTOR", "1")
	if got := tags(false); got != "" {
		t.Errorf("MYGO_INSPECTOR=1 leaves %q", got)
	}
}

func TestWindowsSignCommand(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "My App.exe")
	if err := os.WriteFile(file, []byte("MZ"), 0o644); err != nil {
		t.Fatal(err)
	}
	sign, fail, want := `printf signed >> %1`, "false %1", "MZsigned"
	if runtime.GOOS == "windows" {
		sign, fail, want = `echo signed>> %1`, "exit /b 1 %1", "MZsigned\r\n"
	}
	c := &Config{root: dir, Windows: Windows{SignCommand: sign}}
	if err := signWindows(c, file); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(file); string(b) != want {
		t.Errorf("the sign command got %q", b)
	}
	c.Windows.SignCommand = fail
	if err := signWindows(c, file); err == nil {
		t.Error("a failing sign command succeeded")
	}
}

// peHeaders returns the headers of a PE image, PE32+ or PE32, whose
// certificate table has the given size.
func peHeaders(plus bool, certificates uint32) []byte {
	const pe = 0x40
	opt, dirs, size, magic := pe+24, 96, 224, 0x10b
	if plus {
		dirs, size, magic = 112, 240, 0x20b
	}
	b := make([]byte, opt+size)
	le := binary.LittleEndian
	copy(b, "MZ")
	le.PutUint32(b[0x3c:], pe)
	copy(b[pe:], "PE\x00\x00")
	le.PutUint16(b[pe+20:], uint16(size))
	le.PutUint16(b[opt:], uint16(magic))
	le.PutUint32(b[opt+dirs-4:], 16)
	if certificates != 0 {
		le.PutUint32(b[opt+dirs+4*8:], uint32(len(b)))
		le.PutUint32(b[opt+dirs+4*8+4:], certificates)
	}
	return b
}

func TestPEImage(t *testing.T) {
	dir := t.TempDir()
	files := map[string][]byte{
		"app.exe":    peHeaders(true, 0),
		"signed.exe": peHeaders(true, 0x2a8),
		"x86.dll":    peHeaders(false, 0),
		"signed.dll": peHeaders(false, 0x1f0),
		"dos.exe":    []byte("MZ" + strings.Repeat("\x00", 62)),
		"notes.txt":  []byte("MZ is not enough"),
	}
	for name, b := range files {
		if err := os.WriteFile(filepath.Join(dir, name), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for name, want := range map[string][2]bool{
		"app.exe": {true, false}, "signed.exe": {true, true}, "x86.dll": {true, false}, "signed.dll": {true, true},
		"dos.exe": {false, false}, "notes.txt": {false, false}, "missing.exe": {false, false},
	} {
		path := filepath.Join(dir, name)
		image, signed := peImage(path)
		if image != want[0] || signed != want[1] {
			t.Errorf("peImage(%s) = %v, %v, want %v", name, image, signed, want)
		}
		if !want[0] {
			continue
		}
		// debug/pe finds the same certificate table.
		f, err := pe.Open(path)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		var cert pe.DataDirectory
		switch h := f.OptionalHeader.(type) {
		case *pe.OptionalHeader32:
			cert = h.DataDirectory[pe.IMAGE_DIRECTORY_ENTRY_SECURITY]
		case *pe.OptionalHeader64:
			cert = h.DataDirectory[pe.IMAGE_DIRECTORY_ENTRY_SECURITY]
		}
		f.Close()
		if (cert.Size != 0) != signed {
			t.Errorf("%s: debug/pe finds a certificate table of %d bytes", name, cert.Size)
		}
	}
}

// TestSignWindowsResources signs the executables and libraries among the
// resources that carry no signature, with the sign command.
func TestSignWindowsResources(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	src, dir := t.TempDir(), t.TempDir()
	files := map[string][]byte{
		"bin/tool.exe":      peHeaders(true, 0),
		"bin/vendor.exe":    peHeaders(true, 0x2a8),
		"bin/plugins/x.dll": peHeaders(false, 0),
		"bin/.hidden.dll":   peHeaders(true, 0),
		"data/seed.db":      []byte("MZ, but data"),
		"helper.exe":        peHeaders(true, 0),
		"lib/merged.dll":    peHeaders(false, 0),
	}
	for name, b := range files {
		path := filepath.Join(src, filepath.FromSlash(name))
		os.MkdirAll(filepath.Dir(path), 0o755)
		if err := os.WriteFile(path, b, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	res := []resource{{name: "bin", src: filepath.Join(src, "bin")}, {name: "data", src: filepath.Join(src, "data")}, {name: "helper.exe", src: filepath.Join(src, "helper.exe")},
		{name: "lib"}, {name: "lib/merged.dll", src: filepath.Join(src, "lib", "merged.dll"), nested: true}}
	if err := copyResources(res, dir); err != nil {
		t.Fatal(err)
	}
	c := &Config{root: dir, Windows: Windows{SignCommand: `printf signed >> %1`}}
	if err := signWindowsResources(c, dir, res); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]bool{"bin/tool.exe": true, "bin/plugins/x.dll": true, "helper.exe": true, "lib/merged.dll": true, "bin/vendor.exe": false, "data/seed.db": false} {
		b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		if got := bytes.HasSuffix(b, []byte("signed")); got != want {
			t.Errorf("%s signed: %v, want %v", name, got, want)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "bin", ".hidden.dll")); err == nil {
		t.Error("a hidden file was copied")
	}
	c.Windows.SignCommand = "false %1"
	if err := signWindowsResources(c, dir, res); err == nil {
		t.Error("a failing sign command succeeded")
	}
}

func TestFileAssociations(t *testing.T) {
	c := &Config{Name: "Notes", Identifier: "com.example.notes", Version: "1.0.0", URLSchemes: []string{"notes"},
		FileAssociations: []FileAssociation{
			{Ext: []string{"md", "markdown"}, Name: "Markdown Document", MimeType: "text/markdown"},
			{Ext: []string{"note"}, Name: "Note", Role: "Viewer"},
		}}
	plist := string(infoPlist(c, "Notes", ""))
	for _, want := range []string{"<key>CFBundleDocumentTypes</key>", "<string>Markdown Document</string>", "<string>markdown</string>", "<string>Viewer</string>", "<string>text/markdown</string>"} {
		if !strings.Contains(plist, want) {
			t.Errorf("Info.plist lacks %q", want)
		}
	}
	entry := linuxDesktopEntry(c, "notes", "notes")
	if !strings.Contains(entry, "Exec=notes %U\n") || !strings.Contains(entry, "MimeType=text/markdown;application/x-notes-note;x-scheme-handler/notes;\n") {
		t.Errorf("desktop entry:\n%s", entry)
	}
	xml := mimePackage(c)
	if !strings.Contains(xml, `<mime-type type="application/x-notes-note">`) || !strings.Contains(xml, `<glob pattern="*.note"/>`) || strings.Contains(xml, "text/markdown") {
		t.Errorf("MIME package:\n%s", xml)
	}
	reg, unreg := nsisAssociations(c, "Notes.exe")
	for _, want := range []string{`"Software\Classes\.md\OpenWithProgids" "com.example.notes.md"`, `"Software\Classes\com.example.notes.note\shell\open\command" "" '"$INSTDIR\Notes.exe" "%1"'`, `"Software\Classes\com.example.notes.md\DefaultIcon" "" "$INSTDIR\Notes.exe,0"`, `"Software\Classes\notes" "URL Protocol"`, "SHChangeNotify"} {
		if !strings.Contains(reg, want) {
			t.Errorf("installer registration lacks %s:\n%s", want, reg)
		}
	}
	if !strings.Contains(unreg, `DeleteRegKey HKCU "Software\Classes\com.example.notes.md"`) || !strings.Contains(unreg, `DeleteRegKey HKCU "Software\Classes\notes"`) {
		t.Errorf("installer unregistration:\n%s", unreg)
	}

	dir := t.TempDir()
	for _, bad := range []string{`[{"ext": []}]`, `[{"ext": [".md"]}]`, `[{"ext": ["md"], "role": "Owner"}]`} {
		os.WriteFile(filepath.Join(dir, "mygo.json"), []byte(`{"fileAssociations": `+bad+`}`), 0o644)
		if _, err := loadConfig(dir); err == nil {
			t.Errorf("accepted fileAssociations %s", bad)
		}
	}
}

func TestInfoPlistExtraKeys(t *testing.T) {
	c := &Config{Name: "Cam", Identifier: "com.example.cam", Version: "1.0.0", MacOS: MacOS{
		MinimumSystemVersion: "12.0",
		InfoPlist: map[string]any{
			"NSCameraUsageDescription":             "Scan <documents> & more.",
			"LSUIElement":                          true,
			"NSSupportsAutomaticGraphicsSwitching": false,
			"MyNumbers":                            []any{float64(1), 2.5},
			"MyDict":                               map[string]any{"a": "b"},
		},
	}}
	plist := infoPlist(c, "Cam", "")
	for _, want := range []string{
		"<key>NSCameraUsageDescription</key>\n\t<string>Scan &lt;documents&gt; &amp; more.</string>",
		"<key>LSUIElement</key>\n\t<true/>",
		"<key>NSSupportsAutomaticGraphicsSwitching</key>\n\t<false/>",
		"<integer>1</integer>", "<real>2.5</real>",
	} {
		if !strings.Contains(string(plist), want) {
			t.Errorf("Info.plist lacks %q:\n%s", want, plist)
		}
	}
	if _, err := exec.LookPath("plutil"); err == nil {
		path := filepath.Join(t.TempDir(), "Info.plist")
		os.WriteFile(path, plist, 0o644)
		if out, err := exec.Command("plutil", "-lint", path).CombinedOutput(); err != nil {
			t.Errorf("plutil: %v\n%s", err, out)
		}
	}
}
