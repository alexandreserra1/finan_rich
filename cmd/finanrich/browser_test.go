package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestAppBrowserFindsChromiumOnPath(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("procura pelo PATH só no Linux")
	}
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	if got := appBrowser(); got != "" {
		t.Fatalf("sem navegador no PATH, achou %q", got)
	}
	exe := filepath.Join(dir, "chromium")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := appBrowser(); got != exe {
		t.Fatalf("appBrowser() = %q; quer %q", got, exe)
	}
}
