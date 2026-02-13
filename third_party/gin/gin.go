package gin

import (
	"encoding/json"
	"net/http"
)

type H map[string]any

type HandlerFunc func(*Context)

type Context struct {
	Writer  http.ResponseWriter
	Request *http.Request
}

func (c *Context) JSON(code int, obj any) {
	c.Writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	c.Writer.WriteHeader(code)
	_ = json.NewEncoder(c.Writer).Encode(obj)
}

type Engine struct {
	mux *http.ServeMux
}

func Default() *Engine {
	return &Engine{mux: http.NewServeMux()}
}

func (e *Engine) GET(path string, handler HandlerFunc) {
	e.mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		handler(&Context{Writer: w, Request: r})
	})
}

func (e *Engine) Run(addr string) error {
	return http.ListenAndServe(addr, e.mux)
}

func (e *Engine) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	e.mux.ServeHTTP(w, r)
}
