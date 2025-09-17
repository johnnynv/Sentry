{{/*
Expand the name of the chart.
*/}}
{{- define "sentry.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "sentry.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "sentry.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "sentry.labels" -}}
helm.sh/chart: {{ include "sentry.chart" . }}
{{ include "sentry.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/component: deployment
{{- end }}

{{/*
Selector labels
*/}}
{{- define "sentry.selectorLabels" -}}
app.kubernetes.io/name: {{ include "sentry.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "sentry.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "sentry.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Create the name of the secret to use
*/}}
{{- define "sentry.secretName" -}}
{{- if .Values.secrets.existingSecret }}
{{- .Values.secrets.existingSecret }}
{{- else }}
{{- printf "%s-tokens" (include "sentry.fullname" .) }}
{{- end }}
{{- end }}

{{/*
Create the name of the configmap to use
*/}}
{{- define "sentry.configMapName" -}}
{{- printf "%s-config" (include "sentry.fullname" .) }}
{{- end }}

{{/*
Create the namespace name
*/}}
{{- define "sentry.namespace" -}}
{{- if .Values.namespace.create }}
{{- default .Values.namespace.name .Release.Namespace }}
{{- else }}
{{- .Release.Namespace }}
{{- end }}
{{- end }}

{{/*
Create image name
*/}}
{{- define "sentry.image" -}}
{{- printf "%s:%s" .Values.image.repository (.Values.image.tag | default .Chart.AppVersion) }}
{{- end }}

{{/*
Validate required values
*/}}
{{- define "sentry.validateValues" -}}
{{- if not .Values.config.monitor }}
{{- fail "config.monitor is required" }}
{{- end }}
{{- if not .Values.config.deploy }}
{{- fail "config.deploy is required" }}
{{- end }}
{{- end }}

{{/*
Create command arguments
*/}}
{{- define "sentry.args" -}}
{{- if .Values.args }}
{{- toYaml .Values.args }}
{{- else }}
- "-action=watch"
{{- if .Values.verbose }}
- "-verbose"
{{- end }}
- "-config=/etc/sentry/sentry.yaml"
{{- end }}
{{- end }}
