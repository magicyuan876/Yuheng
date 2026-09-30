// API client for per-user starred resources (DB-backed; see migration
// 000047 and internal/handler/user_resource_favorite.go).
//
// The backend authoritatively scopes favorites to the active (user, tenant)
// pair from the auth context — these helpers therefore never pass user_id
// or tenant_id, and a tenant switch automatically points reads at the
// right namespace.

import { get, post, del } from "@/utils/request";

/**
 * The resource types the web app favorites through this API. The backend
 * accepts more (see types.IsValidFavoriteResourceType); the docs module keeps
 * its own favorites UI.
 */
export const FAVORITE_RESOURCE_TYPES = ["kb"] as const;

export type FavoriteResourceType = (typeof FAVORITE_RESOURCE_TYPES)[number];

export interface FavoriteEntry {
  user_id: string;
  tenant_id: number;
  resource_type: FavoriteResourceType;
  resource_id: string;
  /** ISO timestamp from the server (created_at column). */
  created_at: string;
}

export function listFavorites(type: FavoriteResourceType) {
  return get<{ success: boolean; data: FavoriteEntry[] }>(`/api/v1/user/favorites?type=${encodeURIComponent(type)}`);
}

export function addFavorite(type: FavoriteResourceType, id: string) {
  return post("/api/v1/user/favorites", { type, id });
}

export function removeFavorite(type: FavoriteResourceType, id: string) {
  return del(`/api/v1/user/favorites/${encodeURIComponent(type)}/${encodeURIComponent(id)}`);
}
