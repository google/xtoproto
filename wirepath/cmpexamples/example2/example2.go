package main

import (
	"fmt"
	"reflect"

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
	fmt.Printf("=============== case 1 =================\n")
	case1()
	fmt.Printf("=============== case 2 =================\n")
	case2()
	fmt.Printf("=============== case 3 =================\n")
	case3()
	fmt.Printf("=============== case 4 =================\n")
	case4()
}

func case1() {
	transformCount := 0
	// "The transformer f must be a function "func(T) R" that converts values of
	// type T to those of type R and is implicitly filtered to input values
	// assignable to T. The transformer must not mutate T in any way."
	f := func(i I) string {
		transformCount++
		fmt.Printf("transformIToFoo(%+v) called, got %q\n", i, i.Foo())
		return i.Foo()
	}
	transformIToString := cmp.Transformer("T", f)
	var a1, a2 interface{} = &AlwaysA{}, &AnotherImpl{"A"}

	_, isA1AssignableToI := a1.(I)
	_, isA2AssignableToI := a2.(I)
	explainAssignability("a1", isA1AssignableToI)
	explainAssignability("a2", isA2AssignableToI)
	T := reflect.ValueOf(f).Type().In(0)
	explainAssignability("a1", reflect.ValueOf(a1).Type().AssignableTo(T))
	explainAssignability("a2", reflect.ValueOf(a2).Type().AssignableTo(T))

	fmt.Printf("diff1: %s\n", cmp.Diff(a1, a2, transformIToString))
	fmt.Printf("transform count: %d\n", transformCount)
}

func case2() {
	transformCount := 0
	// "The transformer f must be a function "func(T) R" that converts values of
	// type T to those of type R and is implicitly filtered to input values
	// assignable to T. The transformer must not mutate T in any way."
	f := func(i I) string {
		transformCount++
		fmt.Printf("transformIToFoo(%+v) called, got %q\n", i, i.Foo())
		return i.Foo()
	}
	transformIToString := cmp.Transformer("T", f)
	type X struct{ Eye I }
	var a1, a2 X = X{&AlwaysA{}}, X{&AnotherImpl{"A"}}

	fmt.Printf("diff1: %s\n", cmp.Diff(a1, a2, transformIToString))
	fmt.Printf("transform count: %d\n", transformCount)
}

func case3() {
	transformCount := 0
	// "The transformer f must be a function "func(T) R" that converts values of
	// type T to those of type R and is implicitly filtered to input values
	// assignable to T. The transformer must not mutate T in any way."
	f := func(i I) string {
		transformCount++
		fmt.Printf("transformIToFoo(%+v) called, got %q\n", i, i.Foo())
		return i.Foo()
	}
	transformIToString := cmp.Transformer("T", f)
	type X struct{ Eye interface{} }
	var a1, a2 X = X{&AlwaysA{}}, X{&AnotherImpl{"A"}}

	fmt.Printf("diff1: %s\n", cmp.Diff(a1, a2, transformIToString))
	fmt.Printf("transform count: %d\n", transformCount)
}

func case4() {
	transformCount := 0
	// "The transformer f must be a function "func(T) R" that converts values of
	// type T to those of type R and is implicitly filtered to input values
	// assignable to T. The transformer must not mutate T in any way."
	f := func(i interface{}) string {
		transformCount++
		return i.(I).Foo()
	}
	opt := cmp.FilterValues(func(x, y interface{}) bool {
		_, ok1 := x.(I)
		_, ok2 := y.(I)
		return ok1 && ok2
	}, cmp.Transformer("T", f))
	var a1, a2 interface{} = &AlwaysA{}, &AnotherImpl{"B"}

	fmt.Printf("diff1: %s\n", cmp.Diff(a1, a2, opt))
	fmt.Printf("transform count: %d\n", transformCount)
}

func explainAssignability(name string, assignable bool) {
	if assignable {
		fmt.Printf("%s IS assignable to I\n", name)
		return
	}
	fmt.Printf("%s is NOT assignable to I\n", name)
}

func explainAssignabilityUsingReflection(name string, v reflect.Value) {
	iReflectType := reflect.ValueOf(func(I) {}).Type().In(0)
	assignable := v.Type().AssignableTo(iReflectType)
	if assignable {
		fmt.Printf("%s IS assignable to %v\n", name, iReflectType)
		return
	}
	fmt.Printf("%s is NOT assignable to %v\n", name, iReflectType)
}
