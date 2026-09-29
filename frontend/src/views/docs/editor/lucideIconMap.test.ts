// Every icon name the editor's catalogues ask for has to have a lucide glyph.
//
// The catalogues (slash menu, block menu, selection and table toolbars) are
// data that still speaks in tdesign icon names; lucideIconMap is where those
// names meet the glyphs the new-stack menus draw. A name missing from the map
// does not fail loudly: editorIcon falls back to a generic glyph, and every
// such entry silently wears the same icon. This test is what notices.
import assert from "node:assert/strict";
import { test } from "vitest";

import { blockMenuItems } from "./blockMenu";
import { blockCommands } from "./commands";
import { EDITOR_ICONS } from "./lucideIconMap";
import { TABLE_ACTIONS } from "./tableActions";
import { TOOLBAR_ITEMS } from "./toolbar";

function assertMapped(what: string, names: Iterable<string>) {
  const missing = [...new Set(names)].filter((name) => !(name in EDITOR_ICONS));
  assert.deepEqual(missing, [], `${what} names icons the lucide map lacks: ${missing.join(", ")}`);
}

test("every slash menu icon has a glyph", () => {
  // Built with everything switched on, so no entry is left out of the check.
  const commands = blockCommands({ embeds: true, drawings: true, drawio: true, copyBlockRef: () => {} });
  assertMapped(
    "the slash menu",
    commands.map((c) => c.icon),
  );
});

test("every selection toolbar icon has a glyph", () => {
  assertMapped(
    "the selection toolbar",
    TOOLBAR_ITEMS.map((i) => i.icon),
  );
});

test("every table toolbar icon has a glyph", () => {
  assertMapped(
    "the table toolbar",
    TABLE_ACTIONS.map((a) => a.icon),
  );
});

test("every block menu icon has a glyph", () => {
  assertMapped(
    "the block menu",
    blockMenuItems({ copyBlockRef: () => {} }).map((i) => i.icon),
  );
});
