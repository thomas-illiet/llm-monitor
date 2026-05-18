package runner

import (
	"fmt"
	"sort"
	"sync"
)

// Registry stores tasks by their stable names.
type Registry struct {
	mu    sync.RWMutex
	tasks map[string]Task
}

// NewRegistry creates an empty task registry.
func NewRegistry() *Registry {
	return &Registry{tasks: map[string]Task{}}
}

// Register adds a task by name and rejects invalid or duplicate definitions.
func (r *Registry) Register(task Task) error {
	if task.Name == "" {
		return fmt.Errorf("register task: name is required")
	}
	if task.Handler == nil {
		return fmt.Errorf("register task %q: handler is required", task.Name)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.tasks[task.Name]; exists {
		return fmt.Errorf("register task %q: already registered", task.Name)
	}
	r.tasks[task.Name] = task
	return nil
}

// Get returns a registered task.
func (r *Registry) Get(name string) (Task, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	task, ok := r.tasks[name]
	return task, ok
}

// Names returns registered task names in deterministic order.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.tasks))
	for name := range r.tasks {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
