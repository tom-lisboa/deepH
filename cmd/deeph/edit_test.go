package main

import (
	"reflect"
	"testing"
)

func TestBuildEditRunArgs(t *testing.T) {
	got := buildEditRunArgs("/tmp/ws", true, false, "add helper function")
	want := []string{"--workspace", "/tmp/ws", "--trace", "--coach=false", "coder", "add helper function"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got=%v want=%v", got, want)
	}
}
