package wheelengine

import "testing"

func TestNativeInputFixtures(t *testing.T) {
	g27 := []byte{0, 0x88, 0x0B, 0x41, 0x00, 0x80, 0xFF, 0x80, 0x00, 0x00, 0x00, 0x00}
	p, err := ParseNativeInputReport(ModelG27, g27)
	if err != nil {
		t.Fatal(err)
	}
	if p.Steering != 0x8000 || p.Gear != 1 || !p.PaddleLeft || !p.PaddleRight || !p.HasClutch || p.LayoutID != "g27-c29b-v1" {
		t.Fatalf("g27=%+v", p)
	}
	g25 := make([]byte, 13)
	base := 1
	g25[0] = 0
	g25[base] = 0x08
	g25[base+1] = 0x0B
	g25[base+2] = 0x08
	g25[base+3] = 0
	g25[base+4] = 0x80
	g25[base+5] = 0xff
	g25[base+6] = 0
	g25[base+8] = 80
	g25[base+9] = 220
	g25[base+11] = 0x7f
	p, err = ParseNativeInputReport(ModelG25, g25)
	if err != nil {
		t.Fatal(err)
	}
	if p.Gear != 1 || !p.PaddleLeft || !p.PaddleRight || p.LayoutID != "g25-c299-v1" {
		t.Fatalf("g25=%+v", p)
	}
	dfgt := make([]byte, 12)
	dfgt[0] = 0x08
	dfgt[3] = 0x34
	dfgt[4] = 0x12
	dfgt[5] = 0xff
	dfgt[6] = 0x80
	p, err = ParseNativeInputReport(ModelDFGT, dfgt)
	if err != nil {
		t.Fatal(err)
	}
	if p.Steering != 0x1234 || p.HasClutch || p.LayoutID != "dfgt-c29a-v1" {
		t.Fatalf("dfgt=%+v", p)
	}
}

func TestNativeInputRejectsShortReportsPerAdapter(t *testing.T) {
	for _, m := range []ModelID{ModelG25, ModelG27, ModelDFGT} {
		a, _ := AdapterForModel(m)
		b := make([]byte, a.MinNativeReportBytes-1)
		if _, err := ParseNativeInputReport(m, b); err == nil {
			t.Fatalf("%s accepted short report", m)
		}
	}
}
