package main

import (
	"fmt"

	"github.com/google/go-cmp/cmp"
)

type I interface {
	Foo() string
}

type AlwaysA struct{}

func (a *AlwaysA) Foo() string { return "A" }

type AnotherImpl struct{ foo string }

func (b *AnotherImpl) Foo() string { return b.foo }

func main() {
	transformCount := 0
	// "The transformer f must be a function "func(T) R" that converts values of
	// type T to those of type R and is implicitly filtered to input values
	// assignable to T. The transformer must not mutate T in any way."
	transformIToString := cmp.Transformer("T", func(i I) string {
		transformCount++
		fmt.Printf("transformIToFoo(%+v) called, got %q\n", i, i.Foo())
		return i.Foo()
	})
	var a1, a2 interface{} = &AlwaysA{}, &AnotherImpl{"A"}
	_, isA1AssignableToI := a1.(I)
	_, isA2AssignableToI := a2.(I)
	explainAssignability("a1", isA1AssignableToI)
	explainAssignability("a2", isA2AssignableToI)

	fmt.Printf("diff1: %s\n", cmp.Diff(a1, a2, transformIToString))
	fmt.Printf("transform count: %d\n", transformCount)
}

func explainAssignability(name string, assignable bool) {
	if assignable {
		fmt.Printf("%s IS assignable to I\n", name)
		return
	}
	fmt.Printf("%s is NOT assignable to I\n", name)
}
