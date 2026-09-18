// Bundles src/main.ts (and the shared docs-schema sources it imports) into
// dist/main.js. Dependencies stay external and are installed in the image.
import { build } from 'esbuild'

await build({
  entryPoints: ['src/main.ts'],
  outfile: 'dist/main.js',
  bundle: true,
  platform: 'node',
  target: 'node22',
  format: 'esm',
  packages: 'external',
  sourcemap: true,
  logLevel: 'info',
  banner: {
    js: '// Yuheng collaboration service. MIT licensed; see collab/README.md.',
  },
})
