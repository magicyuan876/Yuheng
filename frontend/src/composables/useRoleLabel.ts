import { type Component } from "vue";
import { CircleUserIcon, EyeIcon, PencilIcon, ShieldCheckIcon } from "@lucide/vue";
import { useI18n } from "vue-i18n";

/**
 * Format a tenant role enum value ('viewer' | 'contributor' | 'admin' | 'owner')
 * into the user-facing label declared under tenantMember.role.* in the i18n
 * bundle. Falls back to the raw role string when the locale does not carry
 * the key — same behaviour as the inline helper this used to live in
 * (UserMenu.vue), now extracted so role-aware UI gates across views can
 * share one implementation.
 *
 * `roleIcon(role)` returns the lucide icon that prefixes the role label (the
 * tenant switcher rows, the login and tenant-switch notifications), or null
 * for a role without one — the caller then renders the label alone.
 */
export function useRoleLabel() {
  const { t } = useI18n();
  const formatRole = (role: string | null | undefined): string => {
    if (!role) return "";
    const key = `tenantMember.role.${role}`;
    const label = t(key);
    return label === key ? role : label;
  };
  const ROLE_ICONS: Record<string, Component> = {
    owner: ShieldCheckIcon,
    admin: CircleUserIcon,
    contributor: PencilIcon,
    viewer: EyeIcon,
  };
  const roleIcon = (role: string | null | undefined): Component | null => (role && ROLE_ICONS[role]) || null;
  return { formatRole, roleIcon };
}
