/**
 * Chunking defaults, mirroring internal/infrastructure/chunker on the backend
 * (DefaultChunkSize, DefaultChunkOverlap, ChunkOverlapOrDefault).
 *
 * A stored chunk_overlap is taken literally: 0 is no overlap. Only an overlap
 * that was never chosen — the field absent — takes the default. Reading it
 * with `|| DEFAULT_CHUNK_OVERLAP` would turn a deliberate 0 back into the
 * default, which is the bug the backend's pointer field exists to prevent.
 */
export const DEFAULT_CHUNK_SIZE = 512;
export const DEFAULT_CHUNK_OVERLAP = 80;

export function chunkOverlapOrDefault(overlap: number | null | undefined): number {
  return overlap ?? DEFAULT_CHUNK_OVERLAP;
}
