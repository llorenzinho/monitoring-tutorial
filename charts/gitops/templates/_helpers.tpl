{{/*
Expand the name of the chart.
*/}}
{{- define "gitops.name" -}}
{{- .Chart.Name }}
{{- end }}

{{/*
Common labels applied to every Application.
*/}}
{{- define "gitops.labels" -}}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}
