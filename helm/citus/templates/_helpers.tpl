{{- /*
Define the base name of the chart.
*/ -}}
{{- define "citus.name" -}}
  {{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- /*
Generate a fully qualified name by combining the chart name and release name.
*/ -}}
{{- define "citus.fullname" -}}
  {{- if eq .Release.Name .Chart.Name -}}
    {{- .Release.Name | trunc 63 | trimSuffix "-" -}}
  {{- else -}}
    {{- printf "%s-%s" (include "citus.name" .) .Release.Name | trunc 63 | trimSuffix "-" -}}
  {{- end -}}
{{- end -}}
