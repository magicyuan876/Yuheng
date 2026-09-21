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
import unusedImports from "eslint-plugin-unused-imports";
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
    plugins: { "unused-imports": unusedImports },
    rules: {
      // Two rules, one job each. The plugin removes unused imports under
      // `--fix`, which the stock rule refuses to do. Everything else that is
      // unused is left to typescript-eslint's rule, because the plugin's own
      // `no-unused-vars` does not understand type positions and reports the
      // parameter names in `defineEmits<{ (e: "x", v: T): void }>()` as
      // unused — several hundred times over. Before `--fix` runs, an unused
      // import is reported by both; after it there is nothing left to
      // report twice.
      "unused-imports/no-unused-imports": "error",
      "@typescript-eslint/no-unused-vars": [
        "error",
        {
          vars: "all",
          // Parameters are not checked. A Vue component's handlers take an
          // event they often ignore, and a callback is shaped by its caller;
          // an unused parameter is a signature, not dead code. Unused
          // variables and imports are dead code, and those are checked.
          args: "none",
          caughtErrors: "none",
          ignoreRestSiblings: true,
          varsIgnorePattern: "^_",
          destructuredArrayIgnorePattern: "^_",
        },
      ],

      // 805 explicit `any`s, every one in code inherited from upstream and
      // none in the docs module. Off here, on below: the new-stack code is
      // held to it, the inherited code is not asked to become typed one
      // warning at a time — it becomes typed when it is rewritten.
      "@typescript-eslint/no-explicit-any": "off",

      // A checker directive is allowed only when it says why. `@ts-ignore`
      // is not allowed at all: it hides an error that may no longer exist,
      // where `@ts-expect-error` fails the build the day it stops being
      // needed.
      "@typescript-eslint/ban-ts-comment": [
        "error",
        {
          "ts-expect-error": "allow-with-description",
          "ts-ignore": true,
          "ts-nocheck": "allow-with-description",
          minimumDescriptionLength: 10,
        },
      ],

      // Route views and a few inherited components are single words
      // (Login, Settings, menu). The rule guards against clashing with HTML
      // element names, which none of these do, and renaming route components
      // ripples through the router for no gain.
      "vue/multi-word-component-names": "off",

      // Five inherited components still carry a plain-JavaScript <script>.
      // Adding lang="ts" is the right fix, but it puts them under vue-tsc,
      // which they have never passed; that is a change to make on purpose,
      // not as a side effect of turning a linter on.
      "vue/block-lang": ["error", { script: { lang: "ts", allowNoLang: true } }],
    },
  },

  {
    // The docs module is where the new stack starts, and it has no `any`
    // today. Keeping it that way is cheaper than getting back to it.
    name: "app/new-stack-strict",
    files: ["src/views/docs/**/*.{ts,vue}"],
    rules: {
      "@typescript-eslint/no-explicit-any": "error",
    },
  },
);
