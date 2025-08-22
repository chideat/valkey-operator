{{ range .packages }}
{{- range .VisibleTypes }}
{{- if .IsExported }}
### {{ .DisplayName }}

{{ .GetComment }}

{{ if .Fields }}
| Field | Description |
| --- | --- |
{{ range .Fields -}}
| `{{ .Name }}` _{{ .Type }}_ | {{ .GetComment }} |
{{ end }}
{{ end }}

{{- end }}
{{- end }}
{{- end }}