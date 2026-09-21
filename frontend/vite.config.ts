import { fileURLToPath, URL } from "node:url";
import { resolve, dirname } from "node:path";
import { existsSync } from "node:fs";
import { execSync } from "node:child_process";
import { createRequire } from "node:module";
import { defineConfig, type Plugin } from "vite";
import vue from "@vitejs/plugin-vue";
import vueJsx from "@vitejs/plugin-vue-jsx";
import tailwindcss from "@tailwindcss/vite";

const __dirname = dirname(fileURLToPath(import.meta.url));
const require = createRequire(import.meta.url);

const pkg = require("./package.json") as { version?: string };
const FRONTEND_VERSION = pkg.version ?? "unknown";

function resolveFrontendCommit(): string {
  const fromEnv = process.env.VITE_FRONTEND_COMMIT || process.env.GITHUB_SHA;
  if (fromEnv) {
    return fromEnv.slice(0, 7);
  }
  try {
    return execSync("git rev-parse --short HEAD", { stdio: ["ignore", "pipe", "ignore"] })
      .toString()
      .trim();
  } catch {
    return "unknown";
  }
}

const FRONTEND_COMMIT = resolveFrontendCommit();

/**
 * Wraps every TDesign stylesheet (components and icons) in the `tdesign`
 * cascade layer.
 *
 * TDesign's components each import their own CSS as a side effect, so the
 * library's styles reach the bundle through JavaScript imports that no CSS
 * file of ours can put inside a layer. Unlayered CSS beats layered CSS
 * regardless of load order, which would leave TDesign overriding every
 * Tailwind utility on the same element. This puts those stylesheets into a
 * layer at the point they are loaded.
 *
 * Each wrapped sheet is preceded by the full layer order. A layer's place
 * is fixed by the first statement that names it, and a TDesign component's
 * CSS can be the first thing in a chunk to do so; repeating the order here
 * means it does not matter which file loads first. The order itself, and
 * why `tdesign` sits where it does, is explained in src/assets/tailwind.css.
 */
const LAYER_ORDER = "@layer theme, base, tdesign, components, utilities;";

function tdesignInLayer(): Plugin {
  return {
    name: "yuheng:tdesign-in-layer",
    enforce: "pre",
    transform(code, id) {
      const [file] = id.split("?");
      if (!file.endsWith(".css") || !/[\\/]node_modules[\\/]tdesign-(?:vue-next|icons-vue-next)[\\/]/.test(file))
        return;
      // @charset is only valid at the very top of a stylesheet, never inside a block.
      const body = code.replace(/^\s*@charset[^;]*;\s*/i, "");
      return { code: `${LAYER_ORDER}\n@layer tdesign {\n${body}\n}`, map: null };
    },
  };
}

const DEV_PROXY_TARGET =
  process.env.VITE_DEV_PROXY_TARGET || process.env.FRONTEND_BACKEND_URL || "http://localhost:8080";

function resolveVueOfficePptxEntry(): string {
  try {
    const pkgDir = dirname(require.resolve("@vue-office/pptx/package.json"));
    const candidates = [
      resolve(pkgDir, "lib/v3/index.js"),
      resolve(pkgDir, "lib/index.js"),
      resolve(pkgDir, "lib/v3/vue-office-pptx.mjs"),
    ];
    const matched = candidates.find((candidate) => existsSync(candidate));
    return matched ?? "@vue-office/pptx";
  } catch {
    return "@vue-office/pptx";
  }
}

export default defineConfig({
  define: {
    __FRONTEND_VERSION__: JSON.stringify(FRONTEND_VERSION),
    __FRONTEND_COMMIT__: JSON.stringify(FRONTEND_COMMIT),
  },
  build: {
    rollupOptions: {
      input: {
        main: resolve(__dirname, "index.html"),
      },
      output: {
        manualChunks(id) {
          if (!id.includes("node_modules")) return;
          // Excalidraw and the React runtime it needs are by a wide margin
          // the largest thing here, and are reached only by somebody who
          // opens a drawing for editing. Keeping them in their own chunk is
          // what stops every reader of every page paying for them.
          if (
            id.includes("@excalidraw") ||
            id.includes("/react-dom/") ||
            id.includes("/react/") ||
            id.includes("/scheduler/")
          ) {
            return "vendor-excalidraw";
          }
          if (id.includes("mermaid") || id.includes("/dagre") || id.includes("cytoscape")) {
            return "vendor-mermaid";
          }
          if (id.includes("marked") || id.includes("katex")) {
            return "vendor-markdown";
          }
          if (id.includes("highlight.js")) {
            return "vendor-highlight";
          }
        },
      },
    },
  },
  plugins: [tdesignInLayer(), vue(), vueJsx(), tailwindcss()],
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
      "@vue-office/pptx": resolveVueOfficePptxEntry(),
    },
  },
  server: {
    port: 5173,
    host: true,
    // 代理配置，用于开发环境
    proxy: {
      "/api": {
        target: DEV_PROXY_TARGET,
        changeOrigin: true,
        secure: false,
      },
      "/files": {
        target: DEV_PROXY_TARGET,
        changeOrigin: true,
        secure: false,
      },
    },
  },
  // `vite preview` 用生产构建产物(dist)本地起服务，是最接近 release 镜像的环境：
  // 同样的压缩 / 拆包 / CSS 加载顺序，可提前暴露只在生产构建出现的问题
  // （如主题变量被打包顺序覆盖）。用法：npm run build && npm run preview
  preview: {
    port: 4173,
    host: true,
    proxy: {
      "/api": {
        target: DEV_PROXY_TARGET,
        changeOrigin: true,
        secure: false,
      },
      "/files": {
        target: DEV_PROXY_TARGET,
        changeOrigin: true,
        secure: false,
      },
    },
  },
});
