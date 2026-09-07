package main

import "testing"

func TestPNGOutputPath(t *testing.T) {
	tests := map[string]string{
		"marked.png":  "marked.png",
		"marked.PNG":  "marked.PNG",
		"marked.jpg":  "marked.png",
		"marked.jpeg": "marked.png",
		"marked":      "marked.png",
	}
	for input, want := range tests {
		if got := pngOutputPath(input); got != want {
			t.Errorf("pngOutputPath(%q) = %q; want %q", input, got, want)
		}
	}
}

func TestEmbedRequiresMessage(t *testing.T) {
	err := embed([]string{"-in", "input.png", "-out", "output.png", "-key", "12345678"})
	if err == nil {
		t.Fatal("embed accepted a missing -message")
	}
}

func TestEmbedRejectsPositionalMessage(t *testing.T) {
	err := embed([]string{"-in", "input.png", "-out", "output.png", "-key", "12345678", "message"})
	if err == nil {
		t.Fatal("embed accepted an unlabelled positional message")
	}
}
