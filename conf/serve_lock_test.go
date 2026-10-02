package conf

import "testing"

func TestTryHoldServeLosesToHolder(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	unlock, err := HoldServe()
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()

	_, ok, err := TryHoldServe()
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("second lock acquired while serve holds it")
	}
}

func TestHoldServeReleasedLetsTheNextCallerIn(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	unlock, err := HoldServe()
	if err != nil {
		t.Fatal(err)
	}
	unlock()

	unlock, ok, err := TryHoldServe()
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("lock still held after release")
	}
	unlock()
}
