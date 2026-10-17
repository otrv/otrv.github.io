package main

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

func TestGoCommentPolicy(t *testing.T) {
	paths, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	files := token.NewFileSet()
	for _, path := range paths {
		file, err := parser.ParseFile(files, path, nil, parser.ParseComments)
		if err != nil {
			t.Fatal(err)
		}
		for _, group := range file.Comments {
			for _, comment := range group.List {
				if strings.HasPrefix(comment.Text, "//go:") || strings.HasPrefix(comment.Text, "//line ") {
					continue
				}
				t.Errorf("%s: replace the comment with a precise name, type, assertion, or behavior test", files.Position(comment.Pos()))
			}
		}
	}
}
