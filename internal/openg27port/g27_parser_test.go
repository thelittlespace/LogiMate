package openg27port

import "testing"

func TestG27ParserBaseline(t *testing.T) {
	report := []byte{0x00, 0x08, 0x00, 0x00, 0x00, 0x80, 0xFF, 0xFF, 0xFF, 0x67, 0x78, 0x98}
	s, err := ParseG27Report(report)
	if err != nil {
		t.Fatal(err)
	}
	if s.Steering != 0x8000 || s.Pedal0 != 0xFF || s.Pedal1 != 0xFF || s.Pedal2 != 0xFF {
		t.Fatalf("unexpected state: %+v", s)
	}
	if s.Buttons != 0x987867 {
		t.Fatalf("buttons=%#x want %#x", s.Buttons, uint64(0x987867))
	}
}

func TestG27ParserSteeringLittleEndian(t *testing.T) {
	s, err := ParseG27Report([]byte{0, 0, 0, 0, 0x34, 0x12, 0xFF, 0xFF, 0xFF})
	if err != nil {
		t.Fatal(err)
	}
	if s.Steering != 0x1234 {
		t.Fatalf("got %#x", s.Steering)
	}
}

func TestG27ParserPedalSemantics(t *testing.T) {
	s, err := ParseG27Report([]byte{0, 0, 0, 0, 0, 0x80, 0x11, 0x22, 0x33})
	if err != nil {
		t.Fatal(err)
	}
	if s.Throttle() != 0x11 || s.Brake() != 0x22 || s.Clutch() != 0x33 {
		t.Fatalf("bad semantic mapping: %+v", s)
	}
}

func TestG27ParserShortReport(t *testing.T) {
	if _, err := ParseG27Report([]byte{0, 0, 0, 0, 0, 0, 0, 0}); err == nil {
		t.Fatal("expected short-report error")
	}
}

func TestG27ButtonPacking(t *testing.T) {
	report := []byte{0, 0, 0, 0, 0, 0x80, 0xFF, 0xFF, 0xFF, 0x01, 0x02, 0x03}
	if got := ExtractG27ButtonBits(report, 9); got != 0x030201 {
		t.Fatalf("got %#x", got)
	}
	if got := ExtractG27ButtonBits([]byte{1, 2, 3}, 9); got != 0 {
		t.Fatalf("past-end got %#x", got)
	}
}
