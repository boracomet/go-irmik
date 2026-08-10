package proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// closeNotifyRecorder satisfies http.CloseNotifier so httputil.ReverseProxy
// works under Gin in tests (httptest.ResponseRecorder alone does not).
type closeNotifyRecorder struct {
	*httptest.ResponseRecorder
	closed chan bool
}

func (r *closeNotifyRecorder) CloseNotify() <-chan bool {
	return r.closed
}

func TestHandlerRewrites(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/x" {
			t.Fatalf("path=%s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer upstream.Close()

	h, err := Handler(Options{Target: upstream.URL, StripPrefix: "/proxy"})
	if err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	r.Any("/proxy/*path", h)
	rec := &closeNotifyRecorder{
		ResponseRecorder: httptest.NewRecorder(),
		closed:           make(chan bool, 1),
	}
	req := httptest.NewRequest(http.MethodGet, "/proxy/api/v1/x", nil)
	r.ServeHTTP(rec, req)
	if rec.Code != 200 || rec.Body.String() != "ok" {
		t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
	}
}
