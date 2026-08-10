package plugin

import "context"

// Hook points for framework extensibility.
type Hook string

const (
	HookBeforeStart Hook = "before_start"
	HookAfterStart  Hook = "after_start"
	HookBeforeStop  Hook = "before_stop"
	HookAfterStop   Hook = "after_stop"
	HookBeforeBuild Hook = "before_build"
	HookAfterBuild  Hook = "after_build"
	HookOnRequest   Hook = "on_request"
)

// Context carries shared state across plugin hooks.
type Context struct {
	Ctx context.Context
	// Values is a simple bag for plugins to share data.
	Values map[string]any
}

func NewContext(ctx context.Context) *Context {
	return &Context{Ctx: ctx, Values: map[string]any{}}
}

// Plugin can register lifecycle hooks.
type Plugin interface {
	Name() string
	Register(r *Registry) error
}

// Handler is invoked for a hook.
type Handler func(c *Context) error

// Registry stores plugins and ordered hook handlers.
type Registry struct {
	plugins  []Plugin
	handlers map[Hook][]Handler
}

func NewRegistry() *Registry {
	return &Registry{handlers: map[Hook][]Handler{}}
}

func (r *Registry) Use(p Plugin) error {
	r.plugins = append(r.plugins, p)
	return p.Register(r)
}

func (r *Registry) On(hook Hook, h Handler) {
	r.handlers[hook] = append(r.handlers[hook], h)
}

func (r *Registry) Run(hook Hook, c *Context) error {
	for _, h := range r.handlers[hook] {
		if err := h(c); err != nil {
			return err
		}
	}
	return nil
}

func (r *Registry) Plugins() []Plugin {
	return append([]Plugin(nil), r.plugins...)
}
