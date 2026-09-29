// Prettier's defaults, with one exception.
//
// Defaults on purpose: they are the formatting every Vue and TypeScript
// example on the internet is written in, so they are what a reader — or a
// model — expects to see and produces without being told. The codebase
// already leaned this way (semicolons and double quotes in the clear
// majority of files), so this is a normalisation, not a restyle.
//
// The one exception is the line width, which matches the backend's `lll`
// setting (.golangci.yml, line-length: 120) so the whole repository wraps
// at the same column.
/** @type {import("prettier").Config} */
export default {
  printWidth: 120,
  // Sorts Tailwind class names in class attributes into the order the
  // compiler emits them; without it every migrated screen lands with its
  // utilities in the order they were typed.
  plugins: ["prettier-plugin-tailwindcss"],
};
