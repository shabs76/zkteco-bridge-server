package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// RegisterManagementRoutes puts the static /employees/export and
// /employees/import alongside the parameterised /employees/:id. gin's router
// panics on some sibling conflicts (see the /iclock catch-all note in
// CLAUDE.md), and it would do so at startup — taking the whole gateway down. This
// pins that the combination registers and resolves the way it is meant to.
func TestManagementRoutesRegisterWithoutConflict(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	defer func() {
		if p := recover(); p != nil {
			t.Fatalf("route registration panicked: %v", p)
		}
	}()
	RegisterManagementRoutes(r, "")

	want := map[string]string{
		"GET /employees":                   "",
		"POST /employees":                  "",
		"GET /employees/export":            "",
		"POST /employees/import":           "",
		"GET /employees/:id":               "",
		"PATCH /employees/:id":             "",
		"DELETE /employees/:id":            "",
		"POST /devices/:id/sync-employees": "",
	}
	for _, ri := range r.Routes() {
		delete(want, ri.Method+" "+ri.Path)
	}
	for missing := range want {
		t.Errorf("route not registered: %s", missing)
	}
}

// The static path must win over the :id parameter, or an export request would be
// handled as "fetch the employee whose id is 'export'".
func TestExportPathIsNotTreatedAsAnID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	var hit string
	r.GET("/employees/export", func(c *gin.Context) { hit = "export"; c.Status(http.StatusOK) })
	r.GET("/employees/:id", func(c *gin.Context) { hit = "byid:" + c.Param("id"); c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/employees/export", nil))
	if hit != "export" {
		t.Fatalf("/employees/export resolved to %q, want the static export handler", hit)
	}

	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/employees/abc-123", nil))
	if hit != "byid:abc-123" {
		t.Fatalf("/employees/abc-123 resolved to %q, want the :id handler", hit)
	}
}
