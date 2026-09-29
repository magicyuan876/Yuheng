import assert from "node:assert/strict";
import { test } from "vitest";
import { mount } from "@vue/test-utils";
import SpaceAvatar from "./SpaceAvatar.vue";

// SpaceAvatar draws a shared space's badge: the first letter of its name on
// a gradient picked from the name, or an emoji. These tests pin the choice
// between the two, the per-size details, and the `space-avatar` hook class
// that ListSpaceSidebar resizes the badge through.

test("renders the upper-cased first letter on a name-derived gradient", () => {
  const w = mount(SpaceAvatar, { props: { name: "research" } });
  assert.ok(w.classes().includes("space-avatar"));
  assert.equal(w.text(), "R");
  assert.match(w.attributes("style") ?? "", /linear-gradient/);
  // The same name always lands on the same gradient.
  const again = mount(SpaceAvatar, { props: { name: "research" } });
  assert.equal(again.attributes("style"), w.attributes("style"));
});

test("an emoji avatar replaces the letter and the decoration", () => {
  const w = mount(SpaceAvatar, { props: { name: "research", avatar: "emoji:🚀" } });
  assert.equal(w.text(), "🚀");
  assert.equal(w.find("svg").exists(), false);
});

test("the small size drops the decoration; the others keep it", () => {
  assert.equal(
    mount(SpaceAvatar, { props: { name: "a", size: "small" } })
      .find("svg")
      .exists(),
    false,
  );
  assert.equal(
    mount(SpaceAvatar, { props: { name: "a" } })
      .find("svg")
      .exists(),
    true,
  );
  assert.equal(
    mount(SpaceAvatar, { props: { name: "a", size: "large" } })
      .find("svg")
      .exists(),
    true,
  );
});

test("an empty name falls back to a question mark", () => {
  assert.equal(mount(SpaceAvatar, { props: { name: "  " } }).text(), "?");
});
