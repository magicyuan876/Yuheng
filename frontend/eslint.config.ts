// The create-vue layout, deliberately: the shape a Vue + TypeScript project
// is expected to have, so anyone — or any model — arriving here recognises
// it without reading further.
//
// Formatting is not linted. Prettier owns it (see prettier.config.js), and
// `skipFormatting` switches off every ESLint rule that would argue with it.
// One tool per job, or the two disagree and both get ignored.
import { globalIgnores } from "eslint/config";
import { defineConfigWithVueTs, vueTsConfigs } from "@vue/eslint-config-typescript";
import pluginVue from "eslint-plugin-vue";
import skipFormatting from "@vue/eslint-config-prettier/skip-formatting";

export default defineConfigWithVueTs(
  {
    name: "app/files-to-lint",
    files: ["**/*.{ts,mts,tsx,vue}"],
  },

  globalIgnores(["**/dist/**", "**/dist-ssr/**", "**/coverage/**", "**/node_modules/**"]),

  // `essential`, not `recommended`: the essential set is the rules whose
  // violations are bugs (unused variables in v-for, duplicate keys, invalid
  // v-model targets); `recommended` adds house-style rules such as attribute
  // order, and on a codebase this size those arrive as thousands of findings
  // nobody reads. Correctness first; taste can be turned on later, one rule
  // at a time, when there is a reason.
  pluginVue.configs["flat/essential"],
  vueTsConfigs.recommended,

  skipFormatting,

  {
    name: "app/rules",
    rules: {
      // The codebase reaches for `any` at a handful of library boundaries
      // (editor chains, third-party message payloads) and is honest about it
      // with a cast. A warning keeps those visible without failing a build
      // over a type the library itself does not provide.
      "@typescript-eslint/no-explicit-any": "warn",
    },
  },
);
