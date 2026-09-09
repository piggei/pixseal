package buildinfo

import (
	"os"
	"strings"
	"testing"
)

func TestVersionFileMatchesBuildInfo(t *testing.T) {
	data, err := os.ReadFile("../../VERSION")
	if err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(string(data))
	want := "PixSeal " + String()
	if got != want {
		t.Fatalf("VERSION=%q, buildinfo=%q", got, want)
	}
}
