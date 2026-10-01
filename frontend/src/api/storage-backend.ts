import { get, post, put, del } from "@/utils/request";

/**
 * How the S3 client addresses a bucket. "auto" (also the empty value) picks
 * virtual-hosted style for amazonaws.com or no endpoint and path style for any
 * other endpoint; "virtual" is required by Aliyun OSS, Tencent COS, Volcengine
 * TOS and Huawei OBS; MinIO and RustFS need "path".
 */
export type S3AddressingStyle = "" | "auto" | "path" | "virtual";

export interface StorageBackendConfig {
  endpoint?: string;
  region?: string;
  access_key_id?: string;
  secret_access_key?: string;
  bucket_name?: string;
  path_prefix?: string;
  use_ssl?: boolean;
  addressing_style?: S3AddressingStyle;
}

export interface StorageBackend {
  id: string;
  tenant_id?: number;
  name: string;
  provider: string;
  config: StorageBackendConfig;
  /**
   * "env" is the deployment's own storage: one read-only row, configured by
   * the environment and shared with every workspace. Everything else is a
   * backend some workspace registered.
   */
  source: "user" | "env";
  status: "active" | "disabled";
  /** 平台共享：所有空间可见可选用，配置与凭据仅系统管理员可见/可改。 */
  is_builtin?: boolean;
  created_at?: string;
  updated_at?: string;
}

export interface StorageBackendListResponse {
  success: boolean;
  data: StorageBackend[];
  /** The workspace default; always set (a new workspace starts on "env"). */
  default_storage_backend_id: string;
}

/**
 * How a knowledge base response names its storage backend: enough to show
 * and pick it, nothing about where it is.
 */
export interface StorageBackendRef {
  id: string;
  name: string;
  provider: string;
  source: "user" | "env";
  is_builtin: boolean;
}

export const listStorageBackends = (): Promise<StorageBackendListResponse> => get("/api/v1/storage-backends");
export const listStorageBackendTypes = (): Promise<{ success: boolean; data: string[] }> =>
  get("/api/v1/storage-backends/types");
export const createStorageBackend = (data: Partial<StorageBackend>) => post("/api/v1/storage-backends", data);
export const updateStorageBackend = (id: string, data: Partial<StorageBackend>) =>
  put(`/api/v1/storage-backends/${id}`, data);
export const deleteStorageBackend = (id: string) => del(`/api/v1/storage-backends/${id}`);
export const setDefaultStorageBackend = (id: string) => put(`/api/v1/storage-backends/${id}/default`, {});
export const testStorageBackend = (data: Partial<StorageBackend>) => post("/api/v1/storage-backends/test", data);
export const testStorageBackendByID = (id: string) => post(`/api/v1/storage-backends/${id}/test`, {});

/**
 * 设置存储实例的平台共享状态。仅系统管理员可调用。
 * 取消共享时，若 owner 之外的空间仍有默认存储、知识库、文档空间或活跃资源绑定，后端返回 400。
 */
export const setStorageBackendSharing = (id: string, shared: boolean) =>
  put(`/api/v1/storage-backends/${id}/sharing`, { shared });
