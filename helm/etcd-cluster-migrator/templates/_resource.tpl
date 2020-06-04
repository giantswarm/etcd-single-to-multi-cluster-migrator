{{/* vim: set filetype=mustache: */}}

{{- define "resource.app.baseDomain" -}}
{{- if hasKey .Values.configmap "baseDomain" }}{{ index .Values.configmap "baseDomain" }}{{ else }}{{ .Values.app.baseDomain }}{{ end }}
{{- end -}}
{{- define "resource.app.registry" -}}
{{- if hasKey .Values.configmap "registry" }}{{ index .Values.configmap "registry" }}{{ else }}{{ .Values.image.registry }}{{ end }}
{{- end -}}
