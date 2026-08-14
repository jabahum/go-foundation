package local

import "testing"

func TestPasswordHasher(t *testing.T) {
	h := NewPasswordHasher()
	hash, err := h.Hash("StrongPass123!")
	if err != nil {
		t.Fatal(err)
	}
	ok, err := h.Verify("StrongPass123!", hash)
	if err != nil || !ok {
		t.Fatalf("expected password to verify: ok=%v err=%v", ok, err)
	}
	ok, err = h.Verify("wrong-password", hash)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("wrong password should not verify")
	}
}
