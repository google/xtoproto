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
	transformIToString := cmp.Transformer("T", func(i I) string {
		transformCount++
		fmt.Printf("transformIToFoo(%+v) called, got %q\n", i, i.Foo())
		return i.Foo()
	})
	{
		var a1, a2 interface{} = &AlwaysA{}, &AnotherImpl{"A"}
		_, isA1AssignableToI := a1.(I)
		_, isA2AssignableToI := a2.(I)
		explainAssignability("a1", isA1AssignableToI)
		explainAssignability("a2", isA2AssignableToI)

		fmt.Printf("diff1: %s\n", cmp.Diff(a1, a2, transformIToString))
		fmt.Printf("transform count: %d", transformCount)
	}
	fmt.Printf("===============================================================================\n")
	{
		var a1, a2 I = &AlwaysA{}, &AnotherImpl{"A"}
		_, isA1AssignableToI := a1.(I)
		_, isA2AssignableToI := a2.(I)
		explainAssignability("a1", isA1AssignableToI)
		explainAssignability("a2", isA2AssignableToI)

		fmt.Printf("diff2: %s\n", cmp.Diff(a1, a2, transformIToString))
	}
	fmt.Printf("===============================================================================\n")
	{
		a1, a2 := &AlwaysA{}, &AnotherImpl{"A"}
		// _, isA1AssignableToI := a1.(I)
		// _, isA2AssignableToI := a2.(I)
		// explainAssignability("a1", isA1AssignableToI)
		// explainAssignability("a2", isA2AssignableToI)

		fmt.Printf("diff3: %s\n", cmp.Diff(a1, a2, transformIToString))
		fmt.Printf("diff4: %s\n", cmp.Diff(&a1, &a2, transformIToString))
	}
}

func explainAssignability(name string, assignable bool) {
	if assignable {
		fmt.Printf("%s IS assignable to I\n", name)
		return
	}
	fmt.Printf("%s is NOT assignable to I\n", name)
}

func explain(name string, thing interface{}) string {
	_, assignable := thing.(I)
	if assignable {
		return fmt.Sprintf("%s IS assignable to I")
	}
	return fmt.Sprintf("%s is NOT assignable to I")
}

func transformIToFoo(i I) string {
	fmt.Printf("transformIToFoo(%+v) called, got %q\n", i, i.Foo())
	return i.Foo()
}
