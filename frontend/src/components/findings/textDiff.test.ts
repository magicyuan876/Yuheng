import assert from "node:assert/strict";
import { describe, it } from "vitest";

import { diffExcerpts, type DiffSegment } from "./textDiff";

const changed = (segments: DiffSegment[]) =>
  segments
    .filter((s) => s.changed)
    .map((s) => s.text)
    .join("|");
const joined = (segments: DiffSegment[]) => segments.map((s) => s.text).join("");

const leave = "员工每年享有15天带薪年假，需提前两周在OA系统提交申请，经直属主管审批后生效。";

describe("diffExcerpts", () => {
  it("marks the changed characters on both sides", () => {
    const other = leave.replace("15", "10");
    const d = diffExcerpts(leave, other);
    assert.equal(changed(d.subject), "5");
    assert.equal(changed(d.related), "0");
    assert.equal(joined(d.subject), leave, "nothing is lost");
    assert.equal(joined(d.related), other);
  });

  it("marks nothing for identical excerpts", () => {
    const d = diffExcerpts(leave, leave);
    assert.equal(changed(d.subject), "");
    assert.equal(joined(d.related), leave);
  });

  it("leaves the edges alone where the passages were cut differently", () => {
    const d = diffExcerpts("上一节讲的是考勤制度。" + leave, leave + "加班需要提前登记。");
    assert.equal(changed(d.subject), "");
    assert.equal(changed(d.related), "");
  });

  it("still marks a change between the anchors", () => {
    const d = diffExcerpts("前文不同。" + leave, "另一份手册。" + leave.replace("两周", "一周"));
    assert.equal(changed(d.subject), "两");
    assert.equal(changed(d.related), "一");
  });

  it("marks nothing when the excerpts share nothing long", () => {
    const d = diffExcerpts(leave, "年假共15天且带薪，提前14天在OA里发起。");
    assert.equal(changed(d.subject), "");
    assert.equal(changed(d.related), "");
  });

  it("copes with empty input and characters outside the BMP", () => {
    assert.deepEqual(diffExcerpts("", ""), { subject: [], related: [] });
    const d = diffExcerpts("😀 the policy says fifteen days of leave", "😀 the policy says ten days of leave");
    assert.equal(joined(d.subject), "😀 the policy says fifteen days of leave");
    assert.ok(changed(d.subject).length > 0);
  });
});
