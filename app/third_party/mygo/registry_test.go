package mygo

import (
	"slices"
	"strings"
	"testing"
)

// Services, events and plugins are registered for the whole process. These
// helpers register them for one test and remove them when it ends, so the
// tests can run again (go test -count).

// bindForTest is BindAs for the duration of t.
func bindForTest(t testing.TB, name string, svc any) {
	t.Helper()
	BindAs(name, svc)
	t.Cleanup(func() { unbind(name) })
}

// newEventForTest is NewEvent for the duration of t.
func newEventForTest[T any](t testing.TB, name string) *Event[T] {
	t.Helper()
	e := NewEvent[T](name)
	t.Cleanup(func() {
		ipc.Lock()
		defer ipc.Unlock()
		ipc.events = slices.DeleteFunc(ipc.events, func(e *eventInfo) bool { return e.name == name })
	})
	return e
}

// useForTest is Use for the duration of t. Plugins that fail are removed
// too.
func useForTest(t testing.TB, p Plugin) {
	t.Helper()
	t.Cleanup(func() {
		plugins.Lock()
		delete(plugins.used, p.Name)
		plugins.Unlock()
		unbind(pluginPrefix + p.Name)
	})
	Use(p)
}

// unbind removes the service name.
func unbind(name string) {
	ipc.Lock()
	defer ipc.Unlock()
	ipc.services = slices.DeleteFunc(ipc.services, func(s *service) bool { return s.name == name })
	for m := range ipc.methods {
		if strings.HasPrefix(m, name+".") {
			delete(ipc.methods, m)
		}
	}
}
