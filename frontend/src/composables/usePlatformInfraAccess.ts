import { computed, type ComputedRef } from 'vue'
import { useAuthStore } from '@/stores/auth'
import { useGovernanceStore } from '@/stores/governance'
import { isPlatformManagedSection } from '@/config/settingsAccess'

/**
 * Whether this user can still reach a shared-infrastructure settings section.
 *
 * Use it to gate the "go to settings" / "add a model" shortcuts embedded in
 * the knowledge-base and agent editors. Under centralised-infrastructure mode
 * those sections are hidden from non-system-admins, so an ungated shortcut
 * would open the settings panel on a section that renders nothing — a dead
 * link pointing at a page the user is not allowed to see.
 *
 * The selectors themselves stay fully functional either way: they read models,
 * vector stores, storage backends and parser engines through their own Viewer+
 * endpoints. Only the "configure it" affordance is conditional.
 *
 * @param section settings section key, e.g. 'models' | 'parser' | 'storage'
 */
export function usePlatformInfraAccess(section: string): ComputedRef<boolean> {
  const authStore = useAuthStore()
  const governance = useGovernanceStore()

  return computed(() => {
    if (!isPlatformManagedSection(section, governance.centralizedInfra)) {
      // Not centralised (or not an infrastructure section): fall back to the
      // workspace role the settings page itself requires.
      return authStore.hasRole('admin') || authStore.canAccessAllTenants
    }
    return authStore.isSystemAdmin || authStore.canAccessAllTenants
  })
}
