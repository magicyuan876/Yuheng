// Word/character counting for the live "N words" display in the page
// header. Mirrors internal/docs/render/extract.go's countWords rune-by-rune
// so the number shown while typing usually matches what the server derives
// once the debounced persist lands -- CJK characters each count as one
// word, a run of Latin letters/digits counts as one word, everything else
// (punctuation, symbols) counts toward character count only.
import type { Node as PMNode } from "@tiptap/pm/model";

const CJK = /[\p{Script=Han}\p{Script=Hiragana}\p{Script=Katakana}\p{Script=Hangul}]/u;
const LETTER_OR_DIGIT = /[\p{L}\p{N}]/u;
const SPACE = /\s/u;

export interface WordCount {
  words: number;
  chars: number;
}

export function countWords(text: string): WordCount {
  let words = 0;
  let chars = 0;
  let inWord = false;
  for (const ch of text) {
    if (CJK.test(ch)) {
      words += 1;
      chars += 1;
      inWord = false;
    } else if (LETTER_OR_DIGIT.test(ch)) {
      if (!inWord) words += 1;
      inWord = true;
      chars += 1;
    } else if (SPACE.test(ch)) {
      inWord = false;
    } else {
      inWord = false;
      chars += 1;
    }
  }
  return { words, chars };
}

/** Word/character count of an entire ProseMirror document. */
export function countDocument(doc: PMNode): WordCount {
  return countWords(doc.textBetween(0, doc.content.size, "\n", "\n"));
}
