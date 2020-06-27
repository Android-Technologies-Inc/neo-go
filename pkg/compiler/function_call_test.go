package compiler_test

import (
	"fmt"
	"math/big"
	"testing"
)

func TestSimpleFunctionCall(t *testing.T) {
	src := `
		package testcase
		func Main() int {
			x := 10
			y := getSomeInteger()
			return x + y
		}

		func getSomeInteger() int {
			x := 10
			return x
		}
	`
	eval(t, src, big.NewInt(20))
}

func TestNotAssignedFunctionCall(t *testing.T) {
	src := `
		package testcase
		func Main() int {
			getSomeInteger()
			getSomeInteger()
			return 0
		}

		func getSomeInteger() int {
			return 0
		}
	`
	// disable stack checks because it is hard right now
	// to distinguish between simple function call traversal
	// and the same traversal inside an assignment.
	evalWithoutStackChecks(t, src, big.NewInt(0))
}

func TestMultipleFunctionCalls(t *testing.T) {
	src := `
		package testcase
		func Main() int {
			x := 10
			y := getSomeInteger()
			return x + y
		}

		func getSomeInteger() int {
			x := 10
			y := getSomeOtherInt()
			return x + y
		}

		func getSomeOtherInt() int {
			x := 8
			return x
		}
	`
	eval(t, src, big.NewInt(28))
}

func TestFunctionCallWithArgs(t *testing.T) {
	src := `
		package testcase
		func Main() int {
			x := 10
			y := getSomeInteger(x)
			return y
		}

		func getSomeInteger(x int) int {
			y := 8
			return x + y
		}
	`
	eval(t, src, big.NewInt(18))
}

func TestFunctionCallWithInterfaceType(t *testing.T) {
	src := `
		package testcase
		func Main() interface{} {
			x := getSomeInteger(10)
			return x
		}

		func getSomeInteger(x interface{}) interface{} {
			return x
		}
	`
	eval(t, src, big.NewInt(10))
}

func TestFunctionCallMultiArg(t *testing.T) {
	src := `
		package testcase
		func Main() int {
			x := addIntegers(2, 4)
			return x
		}

		func addIntegers(x int, y int) int {
			return x + y
		}
	`
	eval(t, src, big.NewInt(6))
}

func TestFunctionWithVoidReturn(t *testing.T) {
	src := `
		package testcase
		func Main() int {
			x := 2
			getSomeInteger()
			y := 4
			return x + y
		}

		func getSomeInteger() { %s }
	`
	t.Run("EmptyBody", func(t *testing.T) {
		src := fmt.Sprintf(src, "")
		eval(t, src, big.NewInt(6))
	})
	t.Run("SingleReturn", func(t *testing.T) {
		src := fmt.Sprintf(src, "return")
		eval(t, src, big.NewInt(6))
	})
}

func TestFunctionWithVoidReturnBranch(t *testing.T) {
	src := `
		package testcase
		func Main() int {
			x := %t
			f(x)
			return 2
		}

		func f(x bool) {
			if x {
				return
			}
		}
	`
	t.Run("ReturnBranch", func(t *testing.T) {
		src := fmt.Sprintf(src, true)
		eval(t, src, big.NewInt(2))
	})
	t.Run("NoReturn", func(t *testing.T) {
		src := fmt.Sprintf(src, false)
		eval(t, src, big.NewInt(2))
	})
}

func TestFunctionWithMultipleArgumentNames(t *testing.T) {
	src := `package foo
	func Main() int {
		return add(1, 2)
	}
	func add(a, b int) int {
		return a + b
	}`
	eval(t, src, big.NewInt(3))
}

func TestLocalsCount(t *testing.T) {
	src := `package foo
	func f(a, b, c int) int {
		sum := a
		for i := 0; i < c; i++ {
			sum += b
		}
		return sum
	}
	func Main() int {
		return f(1, 2, 3)
	}`
	eval(t, src, big.NewInt(7))
}

func TestVariadic(t *testing.T) {
	srcTmpl := `package foo
	func someFunc(a int, b ...int) int {
		sum := a
		for i := range b {
			sum = sum - b[i]
		}
		return sum
	}
	func Main() int {
		%s
		return someFunc(10, %s)
	}`
	t.Run("Elements", func(t *testing.T) {
		src := fmt.Sprintf(srcTmpl, "", "1, 2, 3")
		eval(t, src, big.NewInt(4))
	})
	t.Run("Slice", func(t *testing.T) {
		src := fmt.Sprintf(srcTmpl, "a := []int{1, 2, 3}", "a...")
		eval(t, src, big.NewInt(4))
	})
	t.Run("Literal", func(t *testing.T) {
		src := fmt.Sprintf(srcTmpl, "", "[]int{1, 2, 3}...")
		eval(t, src, big.NewInt(4))
	})
}

func TestVariadicMethod(t *testing.T) {
	src := `package foo
	type myInt int
	func (x myInt) someFunc(a int, b ...int) int {
		sum := int(x) + a
		for i := range b {
			sum = sum - b[i] 
		}
		return sum
	}
	func Main() int {
		x := myInt(38)
		return x.someFunc(10, 1, 2, 3)
	}`
	eval(t, src, big.NewInt(42))
}
