package store

import (
	"context"
	"sort"
	"strings"
	"sync"
)

// Memory is an in-process Store for tests and demos.
type Memory struct {
	mu          sync.RWMutex
	roles       map[string]string   // name → description
	permissions map[string]string   // name → description
	rolePerms   map[string]map[string]struct{}
	userRoles   map[string]map[string]struct{}
}

// NewMemory returns an empty Memory store.
func NewMemory() *Memory {
	return &Memory{
		roles:       make(map[string]string),
		permissions: make(map[string]string),
		rolePerms:   make(map[string]map[string]struct{}),
		userRoles:   make(map[string]map[string]struct{}),
	}
}

// EnsureSchema is a no-op for Memory.
func (m *Memory) EnsureSchema(context.Context) error { return nil }

// RolePermissions implements Store.
func (m *Memory) RolePermissions(context.Context) (map[string][]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[string][]string, len(m.rolePerms))
	for role, set := range m.rolePerms {
		perms := make([]string, 0, len(set))
		for p := range set {
			perms = append(perms, p)
		}
		sort.Strings(perms)
		out[role] = perms
	}
	return out, nil
}

// UserRoles implements Store.
func (m *Memory) UserRoles(context.Context) (map[string][]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[string][]string, len(m.userRoles))
	for userID, set := range m.userRoles {
		roles := make([]string, 0, len(set))
		for role := range set {
			roles = append(roles, role)
		}
		sort.Strings(roles)
		out[userID] = roles
	}
	return out, nil
}

// UpsertRole implements Store.
func (m *Memory) UpsertRole(_ context.Context, name, description string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.roles[name] = description
	return nil
}

// UpsertPermission implements Store.
func (m *Memory) UpsertPermission(_ context.Context, name, description string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.permissions[name] = description
	return nil
}

// Grant implements Store.
func (m *Memory) Grant(_ context.Context, role, permission string) error {
	role = strings.TrimSpace(role)
	permission = strings.TrimSpace(permission)
	if role == "" || permission == "" {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.roles[role]; !ok {
		m.roles[role] = ""
	}
	if _, ok := m.permissions[permission]; !ok {
		m.permissions[permission] = ""
	}
	set, ok := m.rolePerms[role]
	if !ok {
		set = make(map[string]struct{})
		m.rolePerms[role] = set
	}
	set[permission] = struct{}{}
	return nil
}

// AssignRole implements Store.
func (m *Memory) AssignRole(_ context.Context, userID, role string) error {
	userID = strings.TrimSpace(userID)
	role = strings.TrimSpace(role)
	if userID == "" || role == "" {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.roles[role]; !ok {
		m.roles[role] = ""
	}
	set, ok := m.userRoles[userID]
	if !ok {
		set = make(map[string]struct{})
		m.userRoles[userID] = set
	}
	set[role] = struct{}{}
	return nil
}
