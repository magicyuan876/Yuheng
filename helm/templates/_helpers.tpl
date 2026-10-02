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
Return the RustFS image. A digest, when set, is appended to the tag and is
what the runtime actually resolves: the tag stays only so that the reference
still says which release it is.
*/}}
{{- define "yuheng.rustfs.image" -}}
{{- $image := .Values.storage.rustfs.image }}
{{- if $image.digest }}
{{- printf "%s:%s@%s" $image.repository $image.tag $image.digest }}
{{- else }}
{{- printf "%s:%s" $image.repository $image.tag }}
{{- end }}
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
{{- include "yuheng.persistenceStorageClass" (dict "root" . "storageClass" "") }}
{{- end }}

{{/*
Return the storage class of one PVC: its own storageClass when set, otherwise
global.storageClass. "-" asks for the cluster default explicitly (an empty
storageClassName), and leaving both empty omits the field, which also lands on
the default class but lets an admission controller pick it.

Usage: {{ include "yuheng.persistenceStorageClass" (dict "root" . "storageClass" .Values.storage.rustfs.persistence.storageClass) }}
*/}}
{{- define "yuheng.persistenceStorageClass" -}}
{{- $class := .storageClass | default .root.Values.global.storageClass }}
{{- if $class }}
{{- if eq $class "-" }}
storageClassName: ""
{{- else }}
storageClassName: {{ $class | quote }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Validate the storage section, failing at template time on every combination
the app would refuse at start-up or, worse, accept and then lose files with.
Rendered once, from app.yaml.
*/}}
{{- define "yuheng.storage.validate" -}}
{{- $storage := .Values.storage }}
{{- if not (has $storage.type (list "rustfs" "s3" "local")) }}
{{- fail (printf "storage.type must be rustfs, s3 or local, got %q" $storage.type) }}
{{- end }}
{{- if and (eq $storage.type "local") (gt (int .Values.app.replicaCount) 1) }}
{{- fail (printf "storage.type=local keeps files on one ReadWriteOnce volume mounted into one app pod, so it supports app.replicaCount=1 only (got %d); use storage.type=rustfs or s3 to run more replicas" (int .Values.app.replicaCount)) }}
{{- end }}
{{- range $storage.allowList }}
{{- if not (has . (list "local" "s3")) }}
{{- fail (printf "storage.allowList may contain local and s3 only, got %q (rustfs is served through the s3 provider)" .) }}
{{- end }}
{{- end }}
{{- $provider := include "yuheng.storage.provider" . }}
{{- if and $storage.allowList (not (has $provider $storage.allowList)) }}
{{- fail (printf "storage.allowList %v excludes %q, the provider of storage.type=%s; the app would refuse to start" $storage.allowList $provider $storage.type) }}
{{- end }}
{{- if and (has "local" $storage.allowList) (ne $storage.type "local") }}
{{- fail "storage.allowList allows local, but only storage.type=local mounts a volume at the local storage directory; files a workspace stored there would live in the pod's own filesystem and vanish with it" }}
{{- end }}
{{- if eq $storage.type "s3" }}
{{- if not $storage.s3.region }}
{{- fail "storage.s3.region is required for storage.type=s3" }}
{{- end }}
{{- if not $storage.s3.bucket }}
{{- fail "storage.s3.bucket is required for storage.type=s3" }}
{{- end }}
{{- if not (has $storage.s3.addressingStyle (list "auto" "path" "virtual")) }}
{{- fail (printf "storage.s3.addressingStyle must be auto, path or virtual, got %q" $storage.s3.addressingStyle) }}
{{- end }}
{{- if and (not .Values.secrets.existingSecret) (ne (empty .Values.secrets.storageAccessKey) (empty .Values.secrets.storageSecretKey)) }}
{{- fail "secrets.storageAccessKey and secrets.storageSecretKey must be set together, or both left empty to use the AWS default credential chain" }}
{{- end }}
{{- end }}
{{- if and (eq $storage.type "rustfs") (not $storage.s3.bucket) }}
{{- fail "storage.s3.bucket is required: it names the bucket inside the bundled RustFS" }}
{{- end }}
{{- end }}

{{/*
The app's storage provider for storage.type: the bundled RustFS is reached
through the s3 provider like any other S3-compatible service.
*/}}
{{- define "yuheng.storage.provider" -}}
{{- ternary "local" "s3" (eq .Values.storage.type "local") }}
{{- end }}

{{/*
STORAGE_ALLOW_LIST: storage.allowList when given, otherwise the providers this
deployment has somewhere to put files (see storage.allowList in values.yaml).
*/}}
{{- define "yuheng.storage.allowList" -}}
{{- if .Values.storage.allowList }}
{{- join "," .Values.storage.allowList }}
{{- else if eq .Values.storage.type "local" }}
{{- "local,s3" }}
{{- else }}
{{- "s3" }}
{{- end }}
{{- end }}

{{/*
Whether docreader mounts the local storage volume. It reads large videos in
place from it instead of receiving them over gRPC, which only works when there
is one volume both pods can see: storage.type=local on a PVC (an emptyDir
belongs to the app pod alone).
*/}}
{{- define "yuheng.storage.sharedWithDocreader" -}}
{{- if and (eq .Values.storage.type "local") .Values.storage.local.persistence.enabled .Values.docreader.enabled }}true{{ end }}
{{- end }}

{{/*
Annotation for the PVCs that hold stored files. Which of them a release renders
depends on storage.type, so without it a changed storage.type (or an upgrade
that forgets to set it) would have Helm delete the volume holding every file
as soon as the old pod let go of it. Kept volumes outlive the release; delete
them by hand (see "Uninstalling" in README.md).
*/}}
{{- define "yuheng.storage.keepAnnotation" -}}
helm.sh/resource-policy: keep
{{- end }}

{{/*
The local storage volume, for the app (read-write) and docreader (read-only).
*/}}
{{- define "yuheng.storage.localVolume" -}}
- name: data-files
  {{- if .Values.storage.local.persistence.enabled }}
  persistentVolumeClaim:
    claimName: {{ .Values.storage.local.persistence.existingClaim | default (printf "%s-data-files" (include "yuheng.fullname" .)) }}
  {{- else }}
  emptyDir: {}
  {{- end }}
{{- end }}

{{/*
SSRF_WHITELIST_EXTRA. The app's S3 client refuses endpoints that resolve to
private addresses unless they are allow-listed, and a Service name inside the
cluster always does: without its entry the app could not reach its own
storage. So the deployment storage's host is added here, and with virtual-host
addressing its subdomains too (the bucket is a subdomain of the endpoint).
An endpoint operators configured is trusted by construction; app.ssrfWhitelistExtra
carries everything else.
*/}}
{{- define "yuheng.app.ssrfWhitelistExtra" -}}
{{- $hosts := list }}
{{- if eq .Values.storage.type "rustfs" }}
{{- $hosts = append $hosts "rustfs" }}
{{- else if and (eq .Values.storage.type "s3") .Values.storage.s3.endpoint }}
{{- $endpoint := .Values.storage.s3.endpoint | trim }}
{{- if not (contains "://" $endpoint) }}
{{- $endpoint = printf "https://%s" $endpoint }}
{{- end }}
{{- $host := (urlParse $endpoint).hostname }}
{{- if $host }}
{{- $hosts = append $hosts $host }}
{{- if eq .Values.storage.s3.addressingStyle "virtual" }}
{{- $hosts = append $hosts (printf "*.%s" $host) }}
{{- end }}
{{- end }}
{{- end }}
{{- join "," (concat $hosts (.Values.app.ssrfWhitelistExtra | default list) | uniq) }}
{{- end }}

{{/*
The app's storage environment: everything the app reads to describe the
deployment storage (internal/types/storagebackend.go, EnvStorageBackend).
*/}}
{{- define "yuheng.storage.env" -}}
- name: STORAGE_TYPE
  value: {{ include "yuheng.storage.provider" . | quote }}
- name: STORAGE_ALLOW_LIST
  value: {{ include "yuheng.storage.allowList" . | quote }}
{{- if eq .Values.storage.type "local" }}
- name: LOCAL_STORAGE_BASE_DIR
  value: "/data/files"
- name: LOCAL_STORAGE_PATH_PREFIX
  value: {{ .Values.storage.local.pathPrefix | quote }}
{{- if include "yuheng.storage.sharedWithDocreader" . }}
# Where docreader sees the same volume. Set, the app hands large videos over
# as a path instead of streaming their bytes through gRPC.
- name: DOCREADER_SHARED_DATA_DIR
  value: "/data/files"
{{- end }}
{{- else }}
{{- if eq .Values.storage.type "rustfs" }}
# The bundled RustFS: its Service, plain http inside the cluster, and the
# region RustFS answers to. Path-style because the bucket is not a DNS name.
- name: S3_ENDPOINT
  value: {{ printf "http://rustfs:%v" .Values.storage.rustfs.service.port | quote }}
- name: S3_REGION
  value: "us-east-1"
- name: S3_USE_SSL
  value: "false"
- name: S3_ADDRESSING_STYLE
  value: "path"
{{- else }}
- name: S3_ENDPOINT
  value: {{ .Values.storage.s3.endpoint | quote }}
- name: S3_REGION
  value: {{ .Values.storage.s3.region | quote }}
- name: S3_USE_SSL
  value: {{ .Values.storage.s3.useSSL | quote }}
- name: S3_ADDRESSING_STYLE
  value: {{ .Values.storage.s3.addressingStyle | quote }}
{{- end }}
- name: S3_BUCKET_NAME
  value: {{ .Values.storage.s3.bucket | quote }}
- name: S3_PATH_PREFIX
  value: {{ .Values.storage.s3.pathPrefix | quote }}
{{- /*
Credentials. RustFS always has them. An external service may have none, which
selects the AWS default credential chain: then the chart's own Secret carries
no keys and the app is given no reference, while an existingSecret's keys are
optional so that it may leave them out.
*/}}
{{- $withKeys := or (eq .Values.storage.type "rustfs") .Values.secrets.existingSecret .Values.secrets.storageAccessKey }}
{{- if $withKeys }}
{{- $optional := and (eq .Values.storage.type "s3") .Values.secrets.existingSecret }}
- name: S3_ACCESS_KEY
  valueFrom:
    secretKeyRef:
      name: {{ include "yuheng.secretName" . }}
      key: S3_ACCESS_KEY
      {{- if $optional }}
      optional: true
      {{- end }}
- name: S3_SECRET_KEY
  valueFrom:
    secretKeyRef:
      name: {{ include "yuheng.secretName" . }}
      key: S3_SECRET_KEY
      {{- if $optional }}
      optional: true
      {{- end }}
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
