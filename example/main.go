package example

import (
	"fmt"
	"net/http"

	"github.com/Logiraptor/wallhack/router"
)

type b struct {
	Name    string
	Value   int
	Value8  int8
	Value16 int16
	Value32 int32
	Value64 int64
	Bool    bool
}

func (b) Example(method, url, funcName string) interface{} {
	switch funcName {
	case "ReturnBoop":
		return b{
			Name:    "boop",
			Value:   7,
			Value8:  8,
			Value16: 9,
			Value32: 10,
			Value64: 11,
			Bool:    true,
		}
	case "Post":
		return b{
			Name:    "posted boop",
			Value:   7,
			Value8:  8,
			Value16: 9,
			Value32: 10,
			Value64: 11,
			Bool:    true,
		}
	}
	return nil
}

// ReturnBoop does some really cool stuff.
// I'm adding some **markdown** in here for
// style points.
func ReturnBoop(rw http.ResponseWriter, req *http.Request) (b, error) {
	return b{
		Name: req.FormValue("boop"),
	}, nil
}

func Post(rw http.ResponseWriter, req *http.Request) (b, error) {
	return b{
		Value: 3,
	}, fmt.Errorf("stuff went wrong: %d", 2354)
}

func Delete(rw http.ResponseWriter, req *http.Request) (b, error) {
	panic("NOPE")
}

// The application router
var URLs = router.Router{
	router.NewRoute("GET", "/lol", router.Wrap(ReturnBoop)),
	router.NewRoute("POST", "/lol", router.Wrap(Post)),
}

func init() {
	http.Handle("/", URLs.Handler())
}
