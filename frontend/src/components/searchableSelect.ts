/**
 * One entry of a SearchableSelect. `keywords` are extra strings the search
 * matches besides the label — a model's raw name when the label is its
 * display name, say. Callers may extend the type with fields their option
 * slot renders.
 */
export interface SearchableSelectOption {
  value: string;
  label: string;
  keywords?: string[];
}
