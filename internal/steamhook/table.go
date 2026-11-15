package steamhook

import (
	"go/format"
	"slices"
	"strings"
	"text/template"
)

type KnownBuild struct {
	ID      string
	Variant string
	Files   []KnownFile
}

type KnownFile struct {
	Path    string
	SHA256  string
	Package string
}

var rowsTemplate = template.Must(template.New("rows").Parse(`{{range .}}	{
		ID:      {{printf "%q" .ID}},
		Variant: {{printf "%q" .Variant}},
		Files: []KnownFile{
{{- range .Files}}
			{Path: {{printf "%q" .Path}}, SHA256: {{printf "%q" .SHA256}}, Package: {{printf "%q" .Package}}},
{{- end}}
		},
	},
{{end}}`))

func (k KnownBuild) matches(hashes map[string]string) bool {
	for _, file := range k.Files {
		if hashes[file.Path] != file.SHA256 {
			return false
		}
	}
	return len(k.Files) > 0
}

func (k KnownBuild) IsIn(table []KnownBuild) bool {
	hashes := map[string]string{}
	for _, file := range k.Files {
		hashes[file.Path] = file.SHA256
	}
	return slices.ContainsFunc(table, func(known KnownBuild) bool {
		return known.matches(hashes)
	})
}

func RenderRows(builds []KnownBuild) (string, error) {
	var rows strings.Builder
	if err := rowsTemplate.Execute(&rows, builds); err != nil {
		return "", err
	}
	return rows.String(), nil
}

func RenderTable(builds []KnownBuild) ([]byte, error) {
	rows, err := RenderRows(builds)
	if err != nil {
		return nil, err
	}
	return format.Source([]byte("package steamhook\n\nvar KnownBuilds = []KnownBuild{\n" + rows + "}\n"))
}
