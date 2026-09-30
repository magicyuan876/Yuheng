{{/*
Portions Copyright (c) 2025 Tencent.
Modifications Copyright (c) 2026 magicyuan876.
SPDX-License-Identifier: MIT

Yuheng Helm Chart Template Helpers

Best Practices References:
- https://helm.sh/docs/chart_best_practices/templates/
- https://github.com/argoproj/argo-helm/blob/main/charts/argo-cd/templates/_helpers.tpl
*/}}

{{/*
Expand the name of the chart.
*/}}
{{- define "yuheng.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "yuheng.fullname" -}}
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
Ref: https://helm.sh/docs/chart_best_practices/labels/
*/}}
{{- define "yuheng.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels following Kubernetes recommended labels.
Ref: https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/
*/}}
{{- define "yuheng.labels" -}}
helm.sh/chart: {{ include "yuheng.chart" . }}
{{ include "yuheng.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: yuheng
{{- end }}

{{/*
Selector labels
*/}}
{{- define "yuheng.selectorLabels" -}}
app.kubernetes.io/name: {{ include "yuheng.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Component labels - use for individual components
Usage: {{ include "yuheng.componentLabels" (dict "component" "app" "context" .) }}
*/}}
{{- define "yuheng.componentLabels" -}}
{{ include "yuheng.labels" .context }}
app.kubernetes.io/component: {{ .component }}
{{- end }}

{{/*
Component selector labels
Usage: {{ include "yuheng.componentSelectorLabels" (dict "component" "app" "context" .) }}
*/}}
{{- define "yuheng.componentSelectorLabels" -}}
{{ include "yuheng.selectorLabels" .context }}
app.kubernetes.io/component: {{ .component }}
{{- end }}

{{/*
Create the name of the service account to use.
Ref: https://helm.sh/docs/chart_best_practices/rbac/
*/}}
{{- define "yuheng.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "yuheng.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Secret name - supports existing secret
*/}}
{{- define "yuheng.secretName" -}}
{{- if .Values.secrets.existingSecret }}
{{- .Values.secrets.existingSecret }}
{{- else }}
{{- include "yuheng.fullname" . }}-secrets
{{- end }}
{{- end }}

{{/*
Return one of Yuheng's own images (app, frontend, docreader, collab).

Yuheng publishes no images, so there is no default that could be pulled: the
repository is <component>.image.repository when set, otherwise
<global.imageRegistry>/<name>, and rendering fails with instructions when
neither is set. Failing at `helm install` is the point — the alternative was a
chart that installed cleanly and then sat in ImagePullBackOff pointing at a
registry that has nothing in it. The tag defaults to Chart.appVersion, never to
a moving `latest`.

Usage: {{ include "yuheng.ownImage" (dict "root" . "component" "app" "name" "yuheng-app" "image" .Values.app.image) }}
*/}}
{{- define "yuheng.ownImage" -}}
{{- $root := .root }}
{{- $repository := .image.repository }}
{{- if not $repository }}
{{- $registry := $root.Values.global.imageRegistry | default "" | trimSuffix "/" }}
{{- if not $registry }}
{{- fail (printf "set global.imageRegistry (or %s.image.repository): Yuheng publishes no container images, so build them from source and push them to a registry this cluster can pull from; see \"Images\" in helm/README.md" .component) }}
{{- end }}
{{- $repository = printf "%s/%s" $registry .name }}
{{- end }}
{{- printf "%s:%s" $repository (.image.tag | default $root.Chart.AppVersion) }}
{{- end }}

{{- define "yuheng.app.image" -}}
{{- include "yuheng.ownImage" (dict "root" . "component" "app" "name" "yuheng-app" "image" .Values.app.image) }}
{{- end }}

{{- define "yuheng.frontend.image" -}}
{{- include "yuheng.ownImage" (dict "root" . "component" "frontend" "name" "yuheng-ui" "image" .Values.frontend.image) }}
{{- end }}

{{- define "yuheng.docreader.image" -}}
{{- include "yuheng.ownImage" (dict "root" . "component" "docreader" "name" "yuheng-docreader" "image" .Values.docreader.image) }}
{{- end }}

{{- define "yuheng.collab.image" -}}
{{- include "yuheng.ownImage" (dict "root" . "component" "collab" "name" "yuheng-collab" "image" .Values.collab.image) }}
{{- end }}

{{/*
Return the PostgreSQL image with tag.
*/}}
{{- define "yuheng.postgresql.image" -}}
{{- printf "%s:%s" .Values.postgresql.image.repository .Values.postgresql.image.tag }}
{{- end }}

{{/*
Return the Redis image with tag.
*/}}
{{- define "yuheng.redis.image" -}}
{{- printf "%s:%s" .Values.redis.image.repository .Values.redis.image.tag }}
{{- end }}

{{/*
Return the Neo4j image with tag.
*/}}
{{- define "yuheng.neo4j.image" -}}
{{- printf "%s:%s" .Values.neo4j.image.repository .Values.neo4j.image.tag }}
{{- end }}

{{/*
Create image pull secrets list.
*/}}
{{- define "yuheng.imagePullSecrets" -}}
{{- with .Values.global.imagePullSecrets }}
imagePullSecrets:
{{- toYaml . | nindent 2 }}
{{- end }}
{{- end }}

{{/*
Return the storage class name.
*/}}
{{- define "yuheng.storageClass" -}}
{{- if .Values.global.storageClass }}
{{- if eq .Values.global.storageClass "-" }}
storageClassName: ""
{{- else }}
storageClassName: {{ .Values.global.storageClass | quote }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Pod security context.
Merges global defaults with component-specific overrides.
*/}}
{{- define "yuheng.podSecurityContext" -}}
{{- $global := .Values.global.podSecurityContext | default dict }}
{{- $component := .componentSecurityContext | default dict }}
{{- $merged := merge $component $global }}
{{- if $merged }}
securityContext:
{{- toYaml $merged | nindent 2 }}
{{- end }}
{{- end }}

{{/*
Container security context.
*/}}
{{- define "yuheng.containerSecurityContext" -}}
{{- if . }}
securityContext:
{{- toYaml . | nindent 2 }}
{{- end }}
{{- end }}
