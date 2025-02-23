package main

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"testing"

	"golang.org/x/sync/errgroup"
)

func TestSimple(t *testing.T) {
	chTempDir(t)
	var program = []byte(`package main

var t testingDetector

func main() {
	if t.Testing() {
		println("t.Testing()=true")
	} else {
		println("t.Testing()=false")
	}
	println("Hello world!")
}
`)
	if err := os.WriteFile("main.go", program, 0644); err != nil {
		t.Fatal(err)
	}
	var tests = []byte(`package main

import "testing"

func TestMain(t *testing.T) { main() }
`)
	if err := os.WriteFile("main_test.go", tests, 0644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "mod", "init", "example.com/pkg")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go mod init failed: %s\n%s", err, string(out))
	}
	if err := run(); err != nil {
		t.Fatalf("run() = %q, want <nil>", err.Error())
	}
	bin, testbin, err := buildBinaries()
	if err != nil {
		t.Fatal(err)
	}

	if s := "Hello world!"; !bytes.Contains(bin, []byte(s)) {
		t.Errorf("missing %q in program binary", s)
	}
	if s := "t.Testing()=true"; bytes.Contains(bin, []byte(s)) {
		t.Errorf("found %q in program binary", s)
	}
	if s := "t.Testing()=false"; !bytes.Contains(bin, []byte(s)) {
		t.Errorf("missing %q in program binary", s)
	}

	if s := "Hello world!"; !bytes.Contains(testbin, []byte(s)) {
		t.Errorf("missing %q in test binary", s)
	}
	if s := "t.Testing()=true"; !bytes.Contains(testbin, []byte(s)) {
		t.Errorf("missing %q in test binary", s)
	}
	if s := "t.Testing()=false"; bytes.Contains(testbin, []byte(s)) {
		t.Errorf("found %q in test binary", s)
	}
}

func TestTamperDetection(t *testing.T) {
	chTempDir(t)
	var program = []byte(`package main

var t testingDetector

func (t testingDetector) Testing() bool { return true }

func main() {}
`)
	if err := os.WriteFile("main.go", program, 0644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "mod", "init", "example.com/pkg")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go mod init failed: %s\n%s", err, string(out))
	}
	if err := run(); err != nil {
		t.Fatalf("run() = %q, want <nil>", err.Error())
	}
	out, err := exec.Command("go", "run", ".").CombinedOutput()
	if err == nil {
		t.Fatal("go run . successful, want panic")
	} else if ee := new(exec.ExitError); !errors.As(err, &ee) {
		t.Fatalf("go run failed unexpectedly: %s", err)
	}
	wantErr := []byte("bad testingDetector state: got true, want false")
	if !bytes.Contains(out, wantErr) {
		t.Errorf("go run output did not contain %q\n%s", string(wantErr), out)
	}
}

func TestCodeCoverage(t *testing.T) {
	chTempDir(t)
	var program = []byte(`package main

var t testingDetector

func main() {
	if t.Testing() {
		println("t.Testing()=true")
	}
	println("Hello world!")
}
`)
	if err := os.WriteFile("main.go", program, 0644); err != nil {
		t.Fatal(err)
	}
	var tests = []byte(`package main

import "testing"

func TestMain(t *testing.T) { main() }
`)
	if err := os.WriteFile("main_test.go", tests, 0644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "mod", "init", "example.com/pkg")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go mod init failed: %s\n%s", err, string(out))
	}
	if err := run(); err != nil {
		t.Fatalf("run() = %q, want <nil>", err.Error())
	}
	cmd = exec.Command("go", "test", "-cover")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go test failed: %s\n%s", err, out)
	}
	wantOut := []byte("coverage: 100.0% of statements")
	if !bytes.Contains(out, wantOut) {
		t.Errorf("go test output did not contain %q\n%s", string(wantOut), out)
	}
}

func chTempDir(t *testing.T) {
	dir := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("could not get working directory: %s", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("could not change directory to %q: %s", dir, err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) }) // Best effort.
}

func buildBinaries() (bin, testbin []byte, err error) {
	gc := cmp.Or(os.Getenv("GOCOMPILER"), "go")
	var g errgroup.Group
	g.Go(func() error {
		cmd := exec.Command(gc, "build", "-o", "out", ".")
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("go build failed: %w\n%s", err, string(out))
		}
		var err error
		if bin, err = os.ReadFile("out"); err != nil {
			return err
		}
		return nil
	})
	g.Go(func() error {
		cmd := exec.Command(gc, "test", "-c", "-o", "out.test", ".")
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("go test -c failed: %w\n%s", err, string(out))
		}
		var err error
		if testbin, err = os.ReadFile("out.test"); err != nil {
			return err
		}
		return nil
	})
	return bin, testbin, g.Wait()
}
