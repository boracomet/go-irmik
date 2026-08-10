package rbac

import (
	"html/template"

	"github.com/boracomet/go-irmik/irmik/auth"
)

// FuncMap returns template helpers for permission checks.
//
//	tmpl := template.New("app").Funcs(rbac.FuncMap(reg)).ParseFS(...)
//
// Helpers:
//   - Can user perm     → checker.HasPermission(user, perm)
//   - HasRole user role → checker.HasRole(user, role)
func FuncMap(checker Checker) template.FuncMap {
	return template.FuncMap{
		"Can": func(user auth.User, perm string) bool {
			if checker == nil {
				return false
			}
			return checker.HasPermission(user, perm)
		},
		"HasRole": func(user auth.User, role string) bool {
			if checker == nil {
				return false
			}
			return checker.HasRole(user, role)
		},
	}
}
