package money

import "testing"

func TestParse(t *testing.T) {
	cases := map[string]int64{
		"1.234,56":  123456,
		"1234,56":   123456,
		"1234.56":   123456,
		"R$ 10":     1000,
		"0,1":       10,
		"3.500":     350000,
		"1.250.000": 125000000,
		"3.5":       350,
		"19,99":     1999,
	}
	for in, want := range cases {
		got, err := Parse(in)
		if err != nil {
			t.Fatalf("Parse(%q): %v", in, err)
		}
		if got != want {
			t.Errorf("Parse(%q) = %d, want %d", in, got, want)
		}
	}
	if _, err := Parse("-5"); err == nil {
		t.Error("esperava erro para valor negativo")
	}
}

func TestFormat(t *testing.T) {
	cases := map[int64]string{
		0:         "R$ 0,00",
		5:         "R$ 0,05",
		123456:    "R$ 1.234,56",
		100000000: "R$ 1.000.000,00",
		-2550:     "-R$ 25,50",
	}
	for in, want := range cases {
		if got := Format(in); got != want {
			t.Errorf("Format(%d) = %q, want %q", in, got, want)
		}
	}
}
