import { get, post, put, del } from '@/utils/request'

// ===== Types =====

export interface VectorStoreEntity {
  id?: string
  name: string
  engine_type: string
  connection_config: Record<string, any>
  index_config: Record<string, any>
  source: 'env' | 'user'
  readonly: boolean
  tenant_id?: number
  /** 平台共享：所有空间可见可选用，配置与凭据仅系统管理员可见/可改。 */
  is_builtin?: boolean
  created_at?: string
  updated_at?: string
}

export interface VectorStoreTypeInfo {
  type: string
  display_name: string
  connection_fields: FieldSchema[]
  index_fields: FieldSchema[]
}

export interface FieldSchema {
  name: string
  type: 'string' | 'number' | 'boolean'
  required: boolean
  sensitive?: boolean
  description?: string
  default?: any
  // Inclusive bounds for number fields (omitempty on the backend). When
  // absent the UI falls back to per-field heuristics (isReplicaField).
  min?: number
  max?: number
  // Closed value set for string fields (e.g. knn_engine ∈ lucene|faiss).
  // When non-empty the UI renders a select instead of a free-text input.
  enum?: string[]
  // Marks a field that cannot change after store creation. Informational
  // for now (edit mode is fully read-only); kept for forward use.
  immutable?: boolean
}

// ===== API Functions =====

export function listVectorStoreTypes(): Promise<VectorStoreTypeInfo[]> {
  return get('/api/v1/vector-stores/types').then((res: any) => {
    return res.success && res.data ? res.data : []
  })
}

export function listVectorStores(): Promise<{ success: boolean; data: VectorStoreEntity[] }> {
  return get('/api/v1/vector-stores')
}

export function createVectorStore(data: Partial<VectorStoreEntity>) {
  return post('/api/v1/vector-stores', data)
}

export function updateVectorStore(id: string, data: Partial<VectorStoreEntity>) {
  return put(`/api/v1/vector-stores/${id}`, data)
}

export function deleteVectorStore(id: string) {
  return del(`/api/v1/vector-stores/${id}`)
}

export function testVectorStoreRaw(data: { engine_type: string; connection_config: any }): Promise<any> {
  return post('/api/v1/vector-stores/test', data)
}

export function testVectorStoreById(id: string): Promise<any> {
  return post(`/api/v1/vector-stores/${id}/test`, {})
}

/**
 * 设置向量库的平台共享状态。仅系统管理员可调用。
 * 取消共享时，若 owner 之外的空间仍有知识库绑定该向量库，后端返回 400。
 */
export function setVectorStoreSharing(id: string, shared: boolean): Promise<any> {
  return put(`/api/v1/vector-stores/${id}/sharing`, { shared })
}
