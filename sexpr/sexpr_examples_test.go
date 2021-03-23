package sexpr

import (
	"fmt"
)

// Example shows how ReadForm() parses a list s-expression.
func Example() {
	r := NewFileReader("hello-world.sexpr", `("hello-world" 123)`)

	f, err := r.ReadForm()
	if err != nil {
		fmt.Printf("got error: %s", err.Error())
	}
	switch ff := f.(type) {
	case StringForm:
		fmt.Printf("got string: %q", ff.StringValue())
	case ListForm:
		fmt.Printf("got list of %d elements\n", ff.Len())
	}
	fmt.Printf("first element: %q", f.(ListForm).Nth(0).(StringForm).StringValue())
	// Output:
	// got list of 2 elements
	// first element: "hello-world"
}
