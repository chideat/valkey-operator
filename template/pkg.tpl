# API Reference

This document contains the API reference for ValkeyOperator custom resources.

{{ range .packages }}
## {{ .DisplayName }}

{{ .GetComment }}

{{ range .VisibleTypes }}
### {{ .DisplayName }}

{{ .GetComment }}

{{ if .IsExported }}
**Appears in:**
{{- range .References }}
- [{{ .DisplayName }}](#{{ .Anchor }})
{{- end }}
{{- end }}

{{ if .Fields }}
| Field | Description |
| --- | --- |
{{ range .Fields -}}
| `{{ .Name }}` _{{ .Type }}_ | {{ .GetComment }} |
{{ end }}
{{ end }}

{{ end }}
{{ end }}