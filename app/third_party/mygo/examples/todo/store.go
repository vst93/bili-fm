package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
)

// Todo is an item of the list.
type Todo struct {
	ID        int64     `json:"id"`
	Title     string    `json:"title"`
	Done      bool      `json:"done"`
	CreatedAt time.Time `json:"createdAt"`
}

// Filter selects which todos are listed.
type Filter string

// Filters.
const (
	FilterAll    Filter = "all"
	FilterActive Filter = "active"
	FilterDone   Filter = "done"
)

// Stats summarizes the list.
type Stats struct {
	Total  int `json:"total"`
	Active int `json:"active"`
}

// Changed is sent to every window whenever the list changes.
var Changed = mygo.NewEvent[Stats]("todos:changed")

// Todos stores the list as JSON in the user data directory. Its exported
// methods are callable from the frontend.
type Todos struct {
	mu    sync.Mutex
	path  string
	items []Todo
	next  int64
}

// OpenStore loads the saved todos.
func OpenStore() (*Todos, error) {
	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		return nil, err
	}
	t := &Todos{path: filepath.Join(dir, "todos.json")}
	data, err := os.ReadFile(t.path)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &t.items); err != nil {
			return nil, err
		}
	}
	for _, it := range t.items {
		t.next = max(t.next, it.ID)
	}
	return t, nil
}

// List returns the todos matching filter, newest first.
func (t *Todos) List(filter Filter) []Todo {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := []Todo{}
	for _, it := range slices.Backward(t.items) {
		if filter == FilterAll || (filter == FilterDone) == it.Done {
			out = append(out, it)
		}
	}
	return out
}

// Add creates a todo.
func (t *Todos) Add(title string) (Todo, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return Todo{}, errors.New("a todo needs a title")
	}
	t.mu.Lock()
	t.next++
	todo := Todo{ID: t.next, Title: title, CreatedAt: time.Now()}
	t.items = append(t.items, todo)
	t.mu.Unlock()
	return todo, t.save()
}

// Toggle marks a todo done or not done.
func (t *Todos) Toggle(id int64) error {
	return t.update(func() error {
		i := t.index(id)
		if i < 0 {
			return errors.New("todo not found")
		}
		t.items[i].Done = !t.items[i].Done
		return nil
	})
}

// Remove deletes a todo.
func (t *Todos) Remove(id int64) error {
	return t.update(func() error {
		i := t.index(id)
		if i < 0 {
			return errors.New("todo not found")
		}
		t.items = slices.Delete(t.items, i, i+1)
		return nil
	})
}

// ClearDone removes completed todos and returns how many were removed.
func (t *Todos) ClearDone() (int, error) {
	n := 0
	err := t.update(func() error {
		before := len(t.items)
		t.items = slices.DeleteFunc(t.items, func(it Todo) bool { return it.Done })
		n = before - len(t.items)
		return nil
	})
	return n, err
}

// Stats returns counters for the footer.
func (t *Todos) Stats() Stats {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.stats()
}

// Export asks where to save the list and writes it as JSON. It returns the
// chosen path, or "" when the dialog was canceled.
func (t *Todos) Export(ctx context.Context) (string, error) {
	return t.export(mygo.CallerWindow(ctx))
}

func (t *Todos) export(parent *mygo.Window) (string, error) {
	path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
		Parent:      parent,
		Title:       "Export Todos",
		DefaultPath: "todos.json",
		Filters:     []mygo.FileFilter{{Name: "JSON", Extensions: []string{"json"}}},
	})
	if err != nil || path == "" {
		return "", err
	}
	t.mu.Lock()
	data, err := json.MarshalIndent(t.items, "", "  ")
	t.mu.Unlock()
	if err != nil {
		return "", err
	}
	return path, os.WriteFile(path, data, 0o644)
}

func (t *Todos) index(id int64) int {
	return slices.IndexFunc(t.items, func(it Todo) bool { return it.ID == id })
}

func (t *Todos) stats() Stats {
	s := Stats{Total: len(t.items)}
	for _, it := range t.items {
		if !it.Done {
			s.Active++
		}
	}
	return s
}

func (t *Todos) update(fn func() error) error {
	t.mu.Lock()
	err := fn()
	t.mu.Unlock()
	if err != nil {
		return err
	}
	return t.save()
}

// save writes the list and tells every window about the change.
func (t *Todos) save() error {
	t.mu.Lock()
	data, err := json.MarshalIndent(t.items, "", "  ")
	stats := t.stats()
	t.mu.Unlock()
	if err != nil {
		return err
	}
	if err := os.WriteFile(t.path, data, 0o644); err != nil {
		return err
	}
	return Changed.Broadcast(stats)
}
