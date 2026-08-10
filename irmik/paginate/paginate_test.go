package paginate_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/boracomet/go-irmik/irmik/paginate"
)

func TestParseClampAndDefaults(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/?page=0&per_page=999&order=weird&sort=evil", nil)

	p := paginate.Parse(c, paginate.Options{
		DefaultPerPage: 10,
		MaxPerPage:     50,
		DefaultSort:    "created_at",
		DefaultOrder:   "desc",
		SortWhitelist:  []string{"created_at", "title"},
	})
	if p.Page != 1 {
		t.Fatalf("page=%d", p.Page)
	}
	if p.PerPage != 50 {
		t.Fatalf("per_page=%d", p.PerPage)
	}
	if p.Sort != "created_at" {
		t.Fatalf("sort=%q", p.Sort)
	}
	if p.Order != "desc" {
		t.Fatalf("order=%q", p.Order)
	}
}

func TestOffsetLimitOrderBy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/?page=3&per_page=15&sort=title&order=asc&q=hello", nil)

	p := paginate.Parse(c, paginate.Options{
		SortWhitelist: []string{"title", "id"},
	})
	if p.Q != "hello" {
		t.Fatalf("q=%q", p.Q)
	}
	if p.Offset() != 30 || p.Limit() != 15 {
		t.Fatalf("offset=%d limit=%d", p.Offset(), p.Limit())
	}
	ob := p.OrderBy(map[string]string{"title": "items.title", "id": "items.id"})
	if ob != "items.title ASC" {
		t.Fatalf("OrderBy=%q", ob)
	}
	if p.OrderByWhitelist([]string{"id"}) != "" {
		t.Fatal("expected empty when sort not in flat whitelist")
	}
	p.Sort = "id"
	if got := p.OrderByWhitelist([]string{"id"}); got != "id ASC" {
		t.Fatalf("OrderByWhitelist=%q", got)
	}
}

func TestInjectionSafe(t *testing.T) {
	p := paginate.Params{Sort: "title; DROP TABLE", Order: "desc"}
	if p.OrderBy(map[string]string{"title; DROP TABLE": "title; DROP TABLE"}) != "" {
		t.Fatal("unsafe column must be rejected")
	}
	if paginate.FilterColumn("id;delete", []string{"id;delete"}) != "" {
		t.Fatal("unsafe filter column")
	}
	if paginate.FilterColumn("created_at", []string{"created_at"}) != "created_at" {
		t.Fatal("expected allowlisted column")
	}
	if paginate.FilterColumn("secret", []string{"created_at"}) != "" {
		t.Fatal("expected reject")
	}
}
