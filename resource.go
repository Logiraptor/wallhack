package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"reflect"

	"github.com/gorilla/mux"
)

// Router allows registration of handlers
type Router struct {
	routes map[string]http.Handler
}

// Add registers a route
func (r *Router) Add(url string, handler http.Handler) {
	if _, ok := r.routes[url]; ok {
		panic(fmt.Errorf("duplicate paths in router: %s", url))
	}
	r.routes[url] = handler
}

// Handler returns the multiplexer
func (r *Router) Handler() http.Handler {
	m := mux.NewRouter()
	for url, handler := range r.routes {
		m.Handle(url, handler)
	}
	return m
}

// Resource defines an api endpoint
// which switches on the http method
type Resource struct {
	Get    http.Handler
	Post   http.Handler
	Put    http.Handler
	Delete http.Handler
}

// ServeHTTP implements http.Handler
func (r Resource) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case "GET":
		r.Get.ServeHTTP(rw, req)
	case "PUT":
		r.Put.ServeHTTP(rw, req)
	case "POST":
		r.Post.ServeHTTP(rw, req)
	case "DELETE":
		r.Delete.ServeHTTP(rw, req)
	default:
		http.Error(rw, "NO SUCH METHOD", http.StatusNotFound)
	}
}

func wrapMethod(typ reflect.Type, method reflect.Method) http.Handler {
	if err := verifyMethod(method); err != nil {
		panic(fmt.Errorf("%s.%s: %s", typ.String(), method.Name, err.Error()))
	}
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		host := reflect.New(typ).Elem()
		out := method.Func.Call([]reflect.Value{host, reflect.ValueOf(rw), reflect.ValueOf(req)})
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
	Example() interface{}
}

// ToResource takes a struct and converts it into a Resource
// Methods like Get, Post, etc are mapped to the Resource fields.
func ToResource(t interface{}) Resource {
	typ := reflect.TypeOf(t)
	res := Resource{}
	if m, ok := typ.MethodByName("Get"); ok {
		res.Get = wrapMethod(typ, m)
	} else {
		res.Get = http.NotFoundHandler()
	}
	if m, ok := typ.MethodByName("Post"); ok {
		res.Post = wrapMethod(typ, m)
	} else {
		res.Post = http.NotFoundHandler()
	}
	if m, ok := typ.MethodByName("Put"); ok {
		res.Put = wrapMethod(typ, m)
	} else {
		res.Put = http.NotFoundHandler()
	}
	if m, ok := typ.MethodByName("Delete"); ok {
		res.Delete = wrapMethod(typ, m)
	} else {
		res.Delete = http.NotFoundHandler()
	}
	return res
}

func verifyMethod(m reflect.Method) error {
	if m.Type.NumIn() != 3 {
		return fmt.Errorf("method must be of the form (ResponseWriter, *Request)->(T, error)")
	}

	rw := m.Type.In(1)
	if !rw.AssignableTo(reflect.TypeOf((*http.ResponseWriter)(nil)).Elem()) {
		return fmt.Errorf("argument 1 must be an http.ResponseWriter, have %s", rw.String())
	}
	req := m.Type.In(2)
	if !req.AssignableTo(reflect.TypeOf((*http.Request)(nil))) {
		return fmt.Errorf("argument 2 must be an *http.Request, have %s", req.String())
	}

	if m.Type.NumOut() != 2 {
		return fmt.Errorf("method must be of the form (ResponseWriter, *Request)->(T, error)")
	}
	err := m.Type.Out(1)
	if !err.Implements(reflect.TypeOf((*error)(nil)).Elem()) {
		return fmt.Errorf("output 2 must be of type error")
	}

	return nil
}

type b struct {
	Name    string
	Value   int
	Value8  int8
	Value16 int16
	Value32 int32
	Value64 int64
	Bool    bool
}

func (b) Example() interface{} {
	return b{
		Name:    "boop",
		Value:   7,
		Value8:  8,
		Value16: 9,
		Value32: 10,
		Value64: 11,
		Bool:    true,
	}
}

type a struct{}

func (a a) Get(rw http.ResponseWriter, req *http.Request) (b, error) {
	return b{
		Name: req.FormValue("boop"),
	}, nil
}

func (a a) Post(rw http.ResponseWriter, req *http.Request) (b, error) {
	return b{
		Value: 3,
	}, fmt.Errorf("stuff went wrong: %d", 2354)
}

func (a a) Delete(rw http.ResponseWriter, req *http.Request) (b, error) {
	panic("NOPE")
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

func main() {
	r := ToResource(a{})

	http.Handle("/", Recovery(r))
	log.Println(http.ListenAndServe(":"+os.Getenv("PORT"), nil))
}
