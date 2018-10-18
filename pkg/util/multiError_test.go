package util_test

import (
	"errors"
	"testing"

	"github.com/aerogear/keycloak-operator/pkg/util"
)

func TestMultiError(t *testing.T) {
	me := util.NewMultiError()

	me.AddError(errors.New("first error"))
	me.AddError(errors.New("second error"))

	if len(me.GetErrors()) != 2 {
		t.Fatalf("expected 2 errors, got %v", len(me.GetErrors()))
	}

	if me.Error() != "first error: second error" {
		t.Fatalf("expected '%v', got '%v'", "second error: first error", me.Error())
	}
}

func TestMultiErrorAppend(t *testing.T) {
	me := util.NewMultiError()
	other := util.NewMultiError()

	me.AddError(errors.New("me error 1"))

	other.AddError(errors.New("other error 1"))
	other.AddError(errors.New("other error 2"))

	me.AppendMultiErrorer(other)

	if len(me.GetErrors()) != 3 {
		t.Fatalf("expected 3 errors, got %v", len(me.GetErrors()))
	}

	if me.Error() != "me error 1: other error 1: other error 2" {
		t.Fatalf("expected '%v', got '%v'", "other error 2: other error 1: me error 1", me.Error())
	}
}
