// Turning what somebody typed into a comment body, and back.
//
// A comment body is a ProseMirror document so that a mention is a mention
// rather than a string that looks like one. But the box people type into is a
// textarea: mounting a second editor per comment, for what is usually one
// sentence, would cost far more than it returns.
//
// So these two functions are the join, and they have to be exact inverses of
// each other — editing a comment reads the stored body back into the box and
// writes it out again, and anything lost in that round trip is lost from
// somebody's remark without them touching it. That is why this is a module
// with tests rather than two helpers inside a component.
//
// When richer comments are wanted, this is the one place that changes:
// everything downstream already takes a document.

/** A blank line starts a new paragraph; a single newline is a line break. */
const PARAGRAPH_SPLIT = /\n{2,}/;

/** Builds a comment body from typed text. */
export function bodyFromText(text: string): { type: "doc"; content: unknown[] } {
  const paragraphs = text
    .split(PARAGRAPH_SPLIT)
    .map((block) => block.replace(/[ \t]+$/gm, ""))
    .filter((block) => block.trim() !== "");

  return {
    type: "doc",
    content: paragraphs.map((block) => ({
      type: "paragraph",
      content: inlineFromLines(block.split("\n")),
    })),
  };
}

function inlineFromLines(lines: readonly string[]): unknown[] {
  const out: unknown[] = [];
  lines.forEach((line, index) => {
    if (index > 0) out.push({ type: "hardBreak" });
    if (line !== "") out.push({ type: "text", text: line });
  });
  return out;
}

/**
 * Reads a stored body back into text, for editing.
 *
 * Anything it cannot represent — a mention, a page link — comes back as its
 * label rather than disappearing, so re-saving an edited comment does not
 * silently drop what somebody else put in it. The node is lost, but the words
 * are not, and a comment whose mention became plain text is a far smaller
 * surprise than one that quietly shed half its content.
 */
export function textOf(body: unknown): string {
  const blocks = (body as { content?: unknown[] } | null)?.content;
  if (!Array.isArray(blocks)) return "";

  return blocks
    .map((block) => textOfBlock(block))
    .filter((text, index, all) => text !== "" || index < all.length - 1)
    .join("\n\n")
    .trim();
}

function textOfBlock(block: unknown): string {
  const node = block as { type?: string; content?: unknown[] };
  if (!Array.isArray(node.content)) return "";

  return node.content
    .map((child) => {
      const typed = child as {
        type?: string;
        text?: string;
        attrs?: { label?: string | null; userId?: string };
        content?: unknown[];
      };
      switch (typed.type) {
        case "text":
          return typed.text ?? "";
        case "hardBreak":
          return "\n";
        case "mention":
          // The label if there is one; otherwise nothing rather than a raw id,
          // which would be worse than losing the mention.
          return typed.attrs?.label ? `@${typed.attrs.label}` : "";
        default:
          // A nested block (a list item, a quote) contributes its own text.
          return Array.isArray(typed.content) ? textOfBlock(typed) : "";
      }
    })
    .join("");
}
