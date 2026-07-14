package singleserve

import "testing"

func TestShutdownDeniedError(t *testing.T) {
	err := DenyShutdown("work_running", "Wait for work")
	denied, ok := err.(*ShutdownDeniedError)
	if !ok || !denied.valid() || denied.Error() != "Wait for work" {
		t.Fatalf("denial = %#v", err)
	}
	if (&ShutdownDeniedError{Code: "Bad-Code", Message: "message"}).valid() {
		t.Fatal("invalid code accepted")
	}
	if (&ShutdownDeniedError{Code: "valid", Message: ""}).valid() {
		t.Fatal("empty message accepted")
	}
	var nilDenied *ShutdownDeniedError
	if nilDenied.Error() != "shutdown denied" {
		t.Fatalf("nil denial error = %q", nilDenied.Error())
	}
}
