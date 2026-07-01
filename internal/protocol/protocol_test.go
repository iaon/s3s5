package protocol

import "testing"

func TestKeysAndSequences(t *testing.T) {
	if got := OpenKey("/demo/", "abc"); got != "demo/v1/open/abc.json" {
		t.Fatalf("OpenKey = %q", got)
	}
	if got := DataKey("demo", "c2s", "abc", 42); got != "demo/v1/data/c2s/abc/00000000000000000042.bin" {
		t.Fatalf("DataKey = %q", got)
	}
	if got := AckKey("demo", "s2c", "abc"); got != "demo/v1/ack/s2c/abc.json" {
		t.Fatalf("AckKey = %q", got)
	}
	seq, err := ParseSeq("demo/v1/data/c2s/abc/00000000000000000042.bin")
	if err != nil || seq != 42 {
		t.Fatalf("ParseSeq = %d, %v", seq, err)
	}
}

func TestTargetIPv6JSON(t *testing.T) {
	target := Target{Type: AddressIPv6, Host: "2001:db8::1", Port: 443}
	data, err := Marshal(target)
	if err != nil {
		t.Fatal(err)
	}
	var got Target
	if err := Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got != target {
		t.Fatalf("target mismatch: %#v", got)
	}
	if got.Address() != "[2001:db8::1]:443" {
		t.Fatalf("IPv6 address formatting = %q", got.Address())
	}
}
