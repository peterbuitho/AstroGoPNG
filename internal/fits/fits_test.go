package fits

import (
	"testing"

	"github.com/peterbuitho/AstroGoPNG/internal/imageio"
)

func synthFITS(cards []string, data []byte) []byte {
	var b []byte
	for _, c := range cards {
		card := c
		for len(card) < 80 {
			card += " "
		}
		b = append(b, card[:80]...)
	}
	end := "END"
	for len(end) < 80 {
		end += " "
	}
	b = append(b, end...)
	for len(b)%block != 0 {
		b = append(b, ' ')
	}
	b = append(b, data...)
	for len(b)%block != 0 {
		b = append(b, 0)
	}
	return b
}

func TestParseMono8(t *testing.T) {
	f := synthFITS([]string{
		"SIMPLE  =                    T",
		"BITPIX  =                    8",
		"NAXIS   =                    2",
		"NAXIS1  =                    4",
		"NAXIS2  =                    2",
		"OBJECT  = 'Test Target'",
	}, []byte{0, 64, 128, 255, 32, 96, 160, 224})

	raw, err := Parse(f)
	if err != nil {
		t.Fatal(err)
	}
	if raw.Width != 4 || raw.Height != 2 || raw.Channels != 1 || raw.Format != imageio.U8 {
		t.Fatalf("raw = %+v", raw)
	}
	if raw.Object != "Test Target" {
		t.Errorf("object = %q", raw.Object)
	}
	// bottom-up default: row 0 of output is input row 1.
	if raw.Data[0] != 32 || raw.Data[4] != 0 {
		t.Errorf("row flip wrong: %v", raw.Data)
	}
}

func TestStripComment(t *testing.T) {
	if got := stripComment("42 / the answer"); got != "42" {
		t.Errorf("got %q", got)
	}
	if got := stripComment("'M 31'          / object"); got != "'M 31'" {
		t.Errorf("got %q", got)
	}
}
