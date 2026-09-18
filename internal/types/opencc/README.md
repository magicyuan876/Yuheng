# OpenCC t2s dictionaries

`TSPhrases.txt` and `TSCharacters.txt` are the Traditional→Simplified Chinese
dictionaries of the [OpenCC](https://github.com/BYVoid/OpenCC) project
(Open Chinese Convert, © BYVoid and contributors), licensed under the
Apache License 2.0 — the full text is in `LICENSE` next to this file.

The files are copied **unmodified** (byte-for-byte, as shipped in
`github.com/longbridgeapp/opencc v0.3.13`, which mirrors the upstream
`data/dictionary/` files without additions for these two tables). They are
embedded by `internal/types/t2s.go`, which reproduces OpenCC's `t2s`
configuration — maximum matching over the phrase table with a character-table
fallback — so that FAQ question normalisation maps Traditional and Simplified
spellings of the same question onto the same text. Nothing else in the
repository uses them.

Format: one entry per line, `<traditional>\t<simplified>[ <alternative>...]`.
The first candidate is used, as OpenCC does.

If you update these files from a newer OpenCC release, keep this README and the
`LICENSE` file, and record the source version here.
