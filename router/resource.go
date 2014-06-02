package router

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"

	"github.com/gorilla/mux"
)

// HandlerGenerator can turn itself into an http.Handler
type HandlerGenerator interface {
	Handler() http.Handler
}

// Router allows registration of handlers
type Router []Route

// Route defines an http endpoint
type Route struct {
	M, U string
	H    HandlerGenerator
}

// NewRoute creates a Route object
func NewRoute(method, url string, handler HandlerGenerator) Route {
	return Route{M: method, U: url, H: handler}
}

// Handler returns the multiplexer
func (r Router) Handler() http.Handler {
	m := mux.NewRouter()
	for _, handler := range r {
		m.Handle(handler.U, handler.H.Handler()).Methods(handler.M)
	}
	return m
}

// Wrap simply wraps f in a Wrap object
func Wrap(f interface{}) Wrapper {
	return Wrapper{f}
}

// Wrapper turns a function of the form func(ResponseWriter, *Request) (T, error) into a HandlerGenerator
type Wrapper struct {
	F interface{} // must be a function of the form func(ResponseWriter, *Request) (T, error)
}

// Handler implements the HandlerGenerator interface
func (w Wrapper) Handler() http.Handler {
	inner := wrapFunc(reflect.ValueOf(w.F))
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		inner.ServeHTTP(rw, req)
	})
}

func wrapFunc(f reflect.Value) http.Handler {
	if err := verifyFunc(f); err != nil {
		panic(fmt.Errorf("%s: %s", f.String(), err.Error()))
	}
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		out := f.Call([]reflect.Value{reflect.ValueOf(rw), reflect.ValueOf(req)})
		obj, e := out[0].Interface(), out[1].Interface()
		if err, ok := e.(error); ok && err != nil {
			json.NewEncoder(rw).Encode(struct{ Error string }{err.Error()})
			return
		}
		err := json.NewEncoder(rw).Encode(obj)
		if err != nil {
			json.NewEncoder(rw).Encode(struct{ Error string }{err.Error()})
			return
		}
	})
}

// Exampler is something which can produce an example of itself
// This is used by the doc generator to demonstrate
// response bodies.
type Exampler interface {
	Example(method, url, funcName string) interface{}
}

func verifyFunc(m reflect.Value) error {
	typ := m.Type()
	if typ.NumIn() != 2 {
		return fmt.Errorf("func must be of the form func(ResponseWriter, *Request) (T, error)")
	}

	rw := typ.In(0)
	if !rw.AssignableTo(reflect.TypeOf((*http.ResponseWriter)(nil)).Elem()) {
		return fmt.Errorf("argument 1 must be an http.ResponseWriter, have %s", rw.String())
	}
	req := typ.In(1)
	if !req.AssignableTo(reflect.TypeOf((*http.Request)(nil))) {
		return fmt.Errorf("argument 2 must be an *http.Request, have %s", req.String())
	}

	if typ.NumOut() != 2 {
		return fmt.Errorf("func must be of the form func(ResponseWriter, *Request) (T, error)")
	}
	err := typ.Out(1)
	if !err.Implements(reflect.TypeOf((*error)(nil)).Elem()) {
		return fmt.Errorf("output 2 must be of type error")
	}

	return nil
}

// Recovery catches any panics from h and writes a json error
func Recovery(h http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		defer func() {
			if r := recover(); r != nil {
				json.NewEncoder(rw).Encode(struct{ Error string }{fmt.Errorf("PANIC: %v", r).Error()})
			}
		}()
		h.ServeHTTP(rw, req)
	})
}
