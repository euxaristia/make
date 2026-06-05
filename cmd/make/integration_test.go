package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

var testMakeBinary string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "make-test-bin-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	name := "make"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	testMakeBinary = filepath.Join(dir, name)

	cmd := exec.Command("go", "build", "-o", testMakeBinary, ".")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		os.RemoveAll(dir)
		os.Exit(1)
	}

	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func TestIntegrationFixtures(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T) string
		check func(t *testing.T, dir string, stdout string, stderr string, err error)
	}{
		{
			name: "simple file build",
			setup: func(t *testing.T) string {
				return copyFixture(t, "test3")
			},
			check: func(t *testing.T, dir string, stdout string, stderr string, err error) {
				requireExitCode(t, err, 0, stdout, stderr)
				got := readFile(t, filepath.Join(dir, "output.txt"))
				if got != "Hello from make\n" {
					t.Fatalf("output.txt = %q, want %q", got, "Hello from make\n")
				}
			},
		},
		{
			name: "multi-level dependencies",
			setup: func(t *testing.T) string {
				return copyFixture(t, "test2")
			},
			check: func(t *testing.T, dir string, stdout string, stderr string, err error) {
				requireExitCode(t, err, 0, stdout, stderr)
				got := readFile(t, filepath.Join(dir, "program"))
				if got != "main compiled\nutils compiled\n" {
					t.Fatalf("program = %q, want linked object contents", got)
				}
			},
		},
		{
			name: "circular dependency exits with error",
			setup: func(t *testing.T) string {
				return copyFixture(t, "test4")
			},
			check: func(t *testing.T, dir string, stdout string, stderr string, err error) {
				requireExitCode(t, err, 2, stdout, stderr)
				if !strings.Contains(stderr, "Circular dependency") {
					t.Fatalf("stderr = %q, want circular dependency error", stderr)
				}
			},
		},
		{
			name: "c compilation fixture",
			setup: func(t *testing.T) string {
				if _, err := exec.LookPath("gcc"); err != nil {
					t.Skip("gcc not found")
				}
				return copyFixture(t, "test1")
			},
			check: func(t *testing.T, dir string, stdout string, stderr string, err error) {
				requireExitCode(t, err, 0, stdout, stderr)
				if _, err := os.Stat(filepath.Join(dir, "hello")); err != nil {
					t.Fatalf("hello was not built: %v", err)
				}

				stdout, stderr, err = runMake(t, dir)
				requireExitCode(t, err, 0, stdout, stderr)
				if !strings.Contains(stdout, "nothing to be done") {
					t.Fatalf("second run stdout = %q, want up-to-date message", stdout)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := tt.setup(t)
			stdout, stderr, err := runMake(t, dir)
			tt.check(t, dir, stdout, stderr, err)
		})
	}
}

func runMake(t *testing.T, dir string, args ...string) (string, string, error) {
	t.Helper()

	cmd := exec.Command(testMakeBinary, args...)
	cmd.Dir = dir

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

func requireExitCode(t *testing.T, err error, want int, stdout string, stderr string) {
	t.Helper()

	got := exitCode(err)
	if got != want {
		t.Fatalf("exit code = %d, want %d\nstdout:\n%s\nstderr:\n%s", got, want, stdout, stderr)
	}
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}

func copyFixture(t *testing.T, name string) string {
	t.Helper()

	dst := t.TempDir()
	src := filepath.Join("..", "..", "tests", name)

	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, content, info.Mode())
	})
	if err != nil {
		t.Fatal(err)
	}

	return dst
}

func readFile(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}
