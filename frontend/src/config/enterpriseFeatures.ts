import { FileClockIcon, KeyRoundIcon, ShieldCheckIcon, type LucideIcon } from "@lucide/vue";

/**
 * The catalog of features the Enterprise edition adds, as the community
 * frontend presents them.
 *
 * `stage` is the honesty switch. A feature is "planned" until it actually
 * ships; a planned feature is described but never sold (no purchase call to
 * action, no screenshots, no fake controls). When a feature ships, flip its
 * stage here and nowhere else.
 */
export type EnterpriseFeatureStage = "planned" | "available";

export interface EnterpriseFeature {
  /** The extension feature key the backend reports under `extensions`. */
  key: string;
  icon: LucideIcon;
  titleKey: string;
  descriptionKey: string;
  stage: EnterpriseFeatureStage;
}

/**
 * The license management page exists only where the Enterprise edition is
 * installed, so it is not a teaser and stays out of ENTERPRISE_FEATURES; the
 * constant lets code that does exist in that edition refer to the key.
 */
export const ENTERPRISE_LICENSE_FEATURE_KEY = "enterprise.license";

/**
 * Route of the license page for tenant admins, or undefined while the frontend
 * has none (the router has no such route yet). Feature cards for a locked
 * feature show a button only when this is set.
 */
export const ENTERPRISE_LICENSE_ROUTE: string | undefined = undefined;

export const ENTERPRISE_FEATURES: readonly EnterpriseFeature[] = [
  {
    key: "enterprise.acl_retrieval",
    icon: ShieldCheckIcon,
    titleKey: "enterprise.features.aclRetrieval.title",
    descriptionKey: "enterprise.features.aclRetrieval.description",
    stage: "planned",
  },
  {
    key: "enterprise.saml",
    icon: KeyRoundIcon,
    titleKey: "enterprise.features.saml.title",
    descriptionKey: "enterprise.features.saml.description",
    stage: "planned",
  },
  {
    key: "enterprise.audit_export",
    icon: FileClockIcon,
    titleKey: "enterprise.features.auditExport.title",
    descriptionKey: "enterprise.features.auditExport.description",
    stage: "planned",
  },
];

/** i18n keys of the license feature, kept next to its key constant for the Enterprise edition to use. */
export const ENTERPRISE_LICENSE_I18N = {
  titleKey: "enterprise.features.license.title",
  descriptionKey: "enterprise.features.license.description",
} as const;

export function findEnterpriseFeature(key: string): EnterpriseFeature | undefined {
  return ENTERPRISE_FEATURES.find((feature) => feature.key === key);
}
