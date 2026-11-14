package steamhook

import (
	"testing"

	"github.com/ThallesP/macron/internal/failure"
)

func TestParseVDFRoundTrip(t *testing.T) {
	text := `"root"
{
	"plain"		"value"
	"escaped"		"C:\\Games\\\"x\""
	"nested"
	{
		"empty"		""
	}
}
`
	root, err := ParseVDF(text)
	if err != nil {
		t.Fatal(err)
	}
	if got := root.Path("root", "escaped").Value; got != `C:\Games\"x"` {
		t.Fatalf("escaped value is %q", got)
	}
	if root.String() != text {
		t.Fatalf("round trip changed the text:\n%s", root)
	}
}

func TestParseVDFIgnoresCommentsAndCase(t *testing.T) {
	root, err := ParseVDF("// header\n\"Root\" { \"Key\" \"1\" // trailing\n }")
	if err != nil {
		t.Fatal(err)
	}
	if root.Path("root", "key") == nil {
		t.Fatal("keys are not matched case-insensitively")
	}
}

func TestParseVDFRejectsBrokenText(t *testing.T) {
	broken := []string{
		`"root" {`,
		`"root" { "key" }`,
		`}`,
		`"key"`,
		`"root" { "key" "unterminated }`,
	}
	for _, text := range broken {
		_, err := ParseVDF(text)
		assertCode(t, err, failure.CodeInvalidVDF)
	}
}
