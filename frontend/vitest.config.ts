import { fileURLToPath } from "node:url";
import { configDefaults, defineConfig, mergeConfig } from "vitest/config";
import viteConfig from "./vite.config";

// Layered on the Vite config rather than duplicating it, so the `@` alias,
// the Vue plugin and the `define`d build constants are the same ones the
// application is built with. A test that resolves imports differently from
// the build is testing something other than the build.
export default mergeConfig(
  viteConfig,
  defineConfig({
    test: {
      // A DOM for every file, not only the ones that mount a component. The
      // logic-only tests do not need it and do not notice it, and one
      // environment means nobody has to decide, per file, which kind of test
      // they are writing — that decision is where a component test quietly
      // becomes a string-matching test because the DOM was not there.
      environment: "happy-dom",
      // `.mjs` too: sixteen tests inherited from upstream are plain
      // JavaScript. They run under the same runner rather than being left
      // on a second one, because two runners is how one of them stops being
      // run.
      include: ["src/**/*.test.{ts,mjs}"],
      exclude: [...configDefaults.exclude],
      root: fileURLToPath(new URL("./", import.meta.url)),
    },
  }),
);
