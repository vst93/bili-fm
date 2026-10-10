package tsgen

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/egoist/mygo/internal/tsgen/internal/fixture"
)

func modelOf(name string, svc any) Service {
	t := reflect.TypeOf(svc)
	s := Service{Name: name, Type: t}
	ctxType := reflect.TypeFor[context.Context]()
	errType := reflect.TypeFor[error]()
	for i := range t.NumMethod() {
		m := t.Method(i)
		ft := m.Type
		meth := Method{Name: m.Name, Variadic: ft.IsVariadic()}
		start := 1
		if ft.NumIn() > 1 && ft.In(1) == ctxType {
			meth.HasCtx, start = true, 2
		}
		for j := start; j < ft.NumIn(); j++ {
			meth.Params = append(meth.Params, ft.In(j))
		}
		if ft.NumOut() > 0 && ft.Out(0) != errType {
			meth.Result = ft.Out(0)
		}
		if vm, ok := t.Elem().MethodByName(m.Name); ok {
			meth.PC = vm.Func.Pointer()
		} else {
			meth.PC = m.Func.Pointer()
		}
		s.Methods = append(s.Methods, meth)
	}
	return s
}

func generate(t *testing.T) string {
	t.Helper()
	tasks := modelOf("Tasks", &fixture.Tasks{})
	for i, m := range tasks.Methods {
		if m.Name == "Watch" {
			tasks.Methods[i].Channels = []bool{false, true}
		}
	}
	out, err := Generate(Model{
		Services: []Service{tasks},
		Events:   []Event{{Name: "task-added", Type: reflect.TypeFor[fixture.Task]()}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func TestGenerate(t *testing.T) {
	src := generate(t)
	for _, want := range []string{
		`import { call, event, type Channel } from "mygo-runtime";`,
		`export type Status = "todo" | "done";`,
		`export type Priority = 0 | 1 | 2;`,
		"/** Task is a unit of work. */\nexport interface Task {",
		"  id: number;\n",
		"  /** Title of the task. */\n  title: string;\n",
		"  status: Status;\n",
		"  priority: Priority;\n",
		"  tags: string[];\n",
		"  due: string | null;\n",
		"  meta?: Record<string, unknown>;\n",
		"  parent?: Task;\n",
		"  count: string;\n",
		"  raw: unknown;\n",
		"  data: string;\n",
		"  scores: Record<number, number>;\n",
		"  matrix: number[][];\n",
		"  children: (Task | null)[];\n",
		"  extra: { A: boolean };\n",
		"  NoTag: string;\n",
		"  createdAt: string;\n",
		"export interface PageTask {\n  items: Task[];\n  next?: string;\n}",
		"/** Tasks manages tasks. */\nexport const Tasks = {",
		"  /** List returns tasks with the given status. */\n  list(status: Status, limit: number): Promise<PageTask> {\n    return call(\"Tasks.List\", status, limit);\n  },",
		"   * It fails when a task has no title.\n",
		"  add(...tasks: Task[]): Promise<void> {\n    return call(\"Tasks.Add\", ...tasks);\n  },",
		"  count(): Promise<number> {",
		"  delete(arg0: number): Promise<void> {",
		"  watch(status: Status, updates: Channel<Task>): Promise<void> {\n    return call(\"Tasks.Watch\", status, updates);\n  },",
		`  taskAdded: event<Task>("task-added"),`,
	} {
		if !strings.Contains(src, want) {
			t.Errorf("generated code is missing:\n%s", want)
		}
	}
	for _, unwanted := range []string{"Secret", "internal:", "ctx", "function call", "declare global"} {
		if strings.Contains(src, unwanted) {
			t.Errorf("generated code should not contain %q", unwanted)
		}
	}
	if t.Failed() {
		t.Log(src)
	}
}

// TestGeneratedCodeTypeChecks runs tsc on the output, against the
// mygo-runtime package of the workspace, when the repository's dev
// dependencies are installed (bun install).
func TestGeneratedCodeTypeChecks(t *testing.T) {
	tsc, err := filepath.Abs("../../node_modules/.bin/tsc")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(tsc); err != nil {
		t.Skip("tsc not installed; run bun install")
	}
	runtimePkg, err := filepath.Abs("../../packages/runtime")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(runtimePkg, "dist", "index.d.ts")); err != nil {
		t.Skip("mygo-runtime not built; run bun run build")
	}
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "node_modules"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(runtimePkg, filepath.Join(dir, "node_modules", "mygo-runtime")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "client.ts"), []byte(generate(t)), 0o644); err != nil {
		t.Fatal(err)
	}
	usage := `import { Channel, runtime } from "mygo-runtime";
import { Tasks, events, type Task, type Status } from "./client";
const s: Status = "todo";
const page = await Tasks.list(s, 10);
const first: Task | undefined = page.items[0];
await Tasks.add({ id: 1, title: "x", status: "done", priority: 2, tags: [], due: null, count: "1", raw: null, data: "", scores: {}, matrix: [], children: [], extra: { A: true }, NoTag: "", createdAt: "" });
const off = events.taskAdded.on((t) => t.title.toUpperCase());
off();
runtime().window.minimize();
const updates = new Channel<Task>();
const watching = Tasks.watch("todo", updates);
for await (const task of updates) task.title.toUpperCase();
updates.onmessage = (task) => task.id.toFixed();
updates.close();
await watching;
// @ts-expect-error a channel of another type
Tasks.watch("todo", new Channel<string>());
// @ts-expect-error invalid enum value
Tasks.list("nope", 1);
`
	if err := os.WriteFile(filepath.Join(dir, "usage.ts"), []byte(usage), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(tsc, "--noEmit", "--strict", "--noUncheckedIndexedAccess", "--target", "es2022",
		"--module", "esnext", "--moduleResolution", "bundler", "--lib", "es2022,dom", "client.ts", "usage.ts")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("tsc failed: %v\n%s", err, out)
	}
}

func TestNames(t *testing.T) {
	for in, want := range map[string]string{"Greet": "greet", "GetUserByID": "getUserByID", "URLFor": "urlFor", "ID": "id", "already": "already"} {
		if got := camel(in); got != want {
			t.Errorf("camel(%q) = %q, want %q", in, got, want)
		}
	}
	for in, want := range map[string]string{"progress": "progress", "file-changed": "fileChanged", "app:ready": "appReady", "a b": "aB", "123": `"123"`} {
		if got := eventKey(in); got != want {
			t.Errorf("eventKey(%q) = %q, want %q", in, got, want)
		}
	}
	for in, want := range map[string]string{"User": "User", "Page[github.com/x/app.User]": "PageUser", "Pair[main.A,main.B]": "PairAB"} {
		if got := typeName(in); got != want {
			t.Errorf("typeName(%q) = %q, want %q", in, got, want)
		}
	}
	if safeName("new") != "new_" || safeName("x") != "x" {
		t.Error("safeName")
	}
}

func TestValidate(t *testing.T) {
	type bad struct {
		F func()
	}
	type ok struct {
		F func() `json:"-"`
		C chan int
	}
	for _, typ := range []reflect.Type{reflect.TypeFor[chan int](), reflect.TypeFor[bad](), reflect.TypeFor[map[struct{}]int](), reflect.TypeFor[[]complex64]()} {
		if Validate(typ) == nil {
			t.Errorf("Validate(%v) should fail", typ)
		}
	}
	if err := Validate(reflect.TypeFor[fixture.Task]()); err != nil {
		t.Errorf("Validate(Task) = %v", err)
	}
	_ = ok{}
}

func TestStructFieldConflicts(t *testing.T) {
	type A struct{ Name, OnlyA string }
	type B struct {
		Name string
		Tag  string `json:"Name2"`
	}
	type C struct {
		A
		B
		Own string `json:"own"`
	}
	var names []string
	for _, f := range structFields(reflect.TypeFor[C]()) {
		names = append(names, f.name)
	}
	// Name is ambiguous at the same depth and dropped, like encoding/json.
	if got := strings.Join(names, ","); got != "OnlyA,Name2,own" {
		t.Errorf("fields = %s", got)
	}
}
