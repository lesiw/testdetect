# testdetect

Sometimes it is useful in Go code to provide test hooks so that methods can
behave differently at test time. Here is a trivial example.

```go
package main

import "fmt"

var testHookGreet func(string) string

func Greet(s string) string {
    if h := testHookGreet; h != nil {
        return h(s)
    }
    return fmt.Sprintf("Hello, %s!", s)
}

func main() { println(Greet("world")) }
```

Now `testHookGreet` can be set during testing to override the behavior of the
real `Greet()`. This might be useful for tests that are not testing the
implementation of `Greet()` itself, but in which `Greet()` is still being
called somewhere.

Test hooks are found in several places in the [Go standard library][stdlib]
as well as elsewhere [in the wild][search].

The Go compiler is not currently smart enough to recognize that, in the above
example, `testHookGreet` is always `nil` when the real program is running. As a
result, in the finished program, `Greet()` performs a check to see if
`testHookGreet` is `nil` every time it is called ([godbolt][godbolt]).

This has very little impact in the grand scheme of things, especially if the
program is running with [profile-guided optimization][pgo], but it would be
nice for test instrumentation like this to be brought down to zero impact.

`testdetect` generates a package-local `testingDetector` type with a single
method, `Testing()`. While not a true constant, this method is "constant
enough" that most Go compilers will optimize test related branches out of the
finished binary. It's the closest thing to an `#ifdef TEST` as can be achieved
today.

Here it is wired into the previous example.

```go
package main

import "fmt"

var t testingDetector
var testHookGreet func(string) string

func Greet(s string) string {
    if h := testHookGreet; t.Testing() && h != nil {
        return h(s)
    }
    return fmt.Sprintf("Hello, %s!", s)
}

func main() { println(Greet("world")) }
```

Now the entire test hook branch is optimized away, leaving the compiled code
effectively the same as if the program were written like this.

```go
package main

import "fmt"

func Greet(s string) string {
    return fmt.Sprintf("Hello, %s!", s)
}

func main() { println(Greet("world")) }
```

## Usage

Run this in your Go package's directory.

```sh
go run lesiw.io/testdetect@latest
```

Alternatively, include it in your Go code as a `go generate` directive.

```go
//go:generate go run lesiw.io/testdetect@latest
```

This produces two files, `testing_detector.go` and `testing_detector_test.go`.

Write your test-specific code behind a `(testingDetector).Testing()` check.

```go file=main.go
package main

var t testingDetector

func main() {
    if t.Testing() {
        println("t.Testing()=true")
    } else {
        println("t.Testing()=false")
    }
    println("Hello world!")
}
```

## Caveats and details

As of February 2025, checking for `Testing()` in this way correctly strips
test-related branches from Go programs compiled by `gc` (the primary Go
implementation) and `tinygo`. It does not work for `gccgo`.

Technically, this is reliant on implementation details of each of these
compilers, which are not defined in the Go specification and are subject to
change. That said, I find it unlikely that dead code elimination will regress
to the point where these branches are no longer optimized out, and even when
building binaries using versions of the Go toolchain from years ago,
removal of the `if t.Testing() == true` branches has proven consistent.

This generator generates a number of superfluous lines to avoid contributing
negatively to code coverage or tripping up popular linting tools. Since the
entire point of this package is to provide a testing tool that hopefully helps
you improve your own code coverage, generating additional uncovered lines is
considered a bug.

It is theoretically possible to tamper with the value of `t.Testing()`. To
validate that this does not happen, an `init()` function has been added to
`testing_detector.go `that checks to ensure the value of `t.Testing()` is
always equal to `testing.Testing()`, whose value is set at compile time. In
the unlikely event someone were to add the contents of the
`testing_detector_test.go` file into a large codebase, the program would detect
the discrepancy and panic on initialization.

The actual mechanism behind `testingDetector`'s differing behavior between
test and non-test binaries is well-defined in the
[Go spec](https://go.dev/ref/spec). Specifically, it (ab)uses
[selector rules](https://go.dev/ref/spec#Selectors), adding a new method to the
`testingDetector` type that is only present in the `_test.go` file. So while
implementations of dead code elimination may differ from compiler to compiler,
this code is perfectly valid by Go spec rules and will never fail to run.

## A hypothetical better future

I've built this in the hopes that it will someday be retired.

Despite [many](https://github.com/golang/go/issues/12120),
[many](https://github.com/golang/go/issues/14668),
[many](https://github.com/golang/go/issues/21360),
[many](https://github.com/golang/go/issues/60737),
[many](https://github.com/golang/go/issues/60772),
[many](https://github.com/golang/go/issues/64356)
requests and proposals for a compile-time constraint to strip out test-time
specific code, Go still does not have a `test` build tag.
[`testing.Testing()`](https://pkg.go.dev/testing#Testing) was added in Go 1.21
but is sadly not constant, meaning any code gated behind it will still be
present in a release binary.

Almost all of this utility's work could be could be made redundant if Go
treated `_test.go` files the same as any other build constraint by exposing a
`test` build tag.

```go filename=undertest_false.go
//go:build !test
// +build !test
package main

const undertest = false
```

```go filename=undertest_true.go
//go:build test
// +build test
package main

const undertest = true
```

Being a constant declaration, this does not harm code coverage metrics and is
impossible to tamper with: a constant can only be declared once, so declaring
it a second time elsewhere in the code would be a compile error.

I am personally of the opinion that this is simpler, more understandable, less
surprising, and a simple `if false == true` check is highly likely to be
optimized out by any current or future Go compiler.

[stdlib]: https://github.com/search?q=repo%3Agolang%2Fgo%20testhook&type=code
[search]:
    https://github.com/search?q=NOT+repo%3Agolang%2Fgo+%22var+testhook%22+language%3AGo&type=code
[pgo]: https://go.dev/doc/pgo
[godbolt]: https://godbolt.org/z/fYj1rrEEx
