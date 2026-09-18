import unittest
import zipfile
from io import BytesIO

from docreader.parser.epub_parser import EPUBParser
from docreader.parser.registry import registry

_PNG = b"\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR"

_CONTAINER_XML = """<?xml version="1.0" encoding="UTF-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>
"""

_CHAPTER_ONE = (
    "<html><body><h1>Chapter One</h1>"
    "<p>Hello EPUB world.</p>"
    '<p><a href="chapter_02.xhtml#sec2">Chapter 2</a> '
    '<a href="#footnote1">note</a> '
    '<a href="https://example.com">the site</a></p>'
    '<img alt="cover" src="../images/pic.png">'
    "</body></html>"
)

_CHAPTER_TWO = "<html><body><h1>Chapter Two</h1><p>Second chapter text.</p></body></html>"

_NAV = (
    '<html xmlns:epub="http://www.idpf.org/2007/ops"><body>'
    '<nav epub:type="toc"><ol><li><a href="text/chapter_01.xhtml">One</a></li></ol></nav>'
    "</body></html>"
)


def _opf(spine_ids, nav=True):
    items = [
        '<item id="ch1" href="text/chapter_01.xhtml" media-type="application/xhtml+xml"/>',
        '<item id="ch2" href="text/chapter_02.xhtml" media-type="application/xhtml+xml"/>',
        '<item id="pic" href="images/pic.png" media-type="image/png"/>',
    ]
    if nav:
        items.append(
            '<item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>'
        )
    itemrefs = "".join(f'<itemref idref="{i}"/>' for i in spine_ids)
    return f"""<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="uid">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:identifier id="uid">test-epub</dc:identifier>
    <dc:title>Tiny EPUB</dc:title>
    <dc:language>en</dc:language>
    <dc:creator>Yuheng</dc:creator>
    <dc:creator>Second Author</dc:creator>
  </metadata>
  <manifest>{"".join(items)}</manifest>
  <spine>{itemrefs}</spine>
</package>
"""


def _epub_bytes(spine_ids=("nav", "ch1", "ch2"), with_container=True, with_opf=True):
    buf = BytesIO()
    with zipfile.ZipFile(buf, "w") as z:
        z.writestr("mimetype", "application/epub+zip", compress_type=zipfile.ZIP_STORED)
        if with_container:
            z.writestr("META-INF/container.xml", _CONTAINER_XML)
        if with_opf:
            z.writestr("OEBPS/content.opf", _opf(spine_ids))
        z.writestr("OEBPS/nav.xhtml", _NAV)
        z.writestr("OEBPS/text/chapter_01.xhtml", _CHAPTER_ONE)
        z.writestr("OEBPS/text/chapter_02.xhtml", _CHAPTER_TWO)
        z.writestr("OEBPS/images/pic.png", _PNG)
    return buf.getvalue()


def _parse(content, **kwargs):
    return EPUBParser(file_name="tiny.epub", file_type="epub", **kwargs).parse_into_text(
        content
    )


class EPUBParserTest(unittest.TestCase):
    def test_parse_minimal_epub(self):
        document = _parse(_epub_bytes())

        self.assertIn("Hello EPUB world", document.content)
        self.assertEqual(document.metadata["source_format"], "epub")
        self.assertEqual(document.metadata["title"], "Tiny EPUB")
        self.assertEqual(document.metadata["author"], "Yuheng, Second Author")
        self.assertEqual(document.metadata["language"], "en")
        self.assertEqual(document.metadata["chapter_count"], 2)
        self.assertEqual(len(document.images), 1)
        image_ref = next(iter(document.images))
        self.assertTrue(image_ref.startswith("images/"))
        self.assertIn(image_ref, document.content)
        self.assertNotIn("../images/pic.png", document.content)

    def test_nav_document_is_not_emitted_as_a_chapter(self):
        document = _parse(_epub_bytes())

        self.assertNotIn("## Nav", document.content)
        self.assertTrue(document.content.startswith("## Chapter One"))

    def test_spine_order_wins_over_file_names(self):
        document = _parse(_epub_bytes(spine_ids=("ch2", "ch1")))

        self.assertLess(
            document.content.index("Second chapter text"),
            document.content.index("Hello EPUB world"),
        )

    def test_internal_links_are_unwrapped_but_external_links_remain(self):
        document = _parse(_epub_bytes())

        self.assertIn("Chapter 2", document.content)
        self.assertIn("note", document.content)
        self.assertNotIn("chapter_02.xhtml#sec2", document.content)
        self.assertNotIn("#footnote1", document.content)
        self.assertIn("[the site](https://example.com)", document.content)

    def test_parse_without_images(self):
        document = _parse(_epub_bytes(), extract_images=False)

        self.assertEqual(document.images, {})

    def test_archive_without_opf_falls_back_to_html_scan(self):
        document = _parse(_epub_bytes(with_container=False, with_opf=False))

        self.assertIn("Hello EPUB world", document.content)
        self.assertIn("Second chapter text", document.content)
        self.assertLess(
            document.content.index("Hello EPUB world"),
            document.content.index("Second chapter text"),
        )
        self.assertEqual(len(document.images), 1)

    def test_registry_resolves_epub(self):
        self.assertIs(registry.get_parser_class("", "epub"), EPUBParser)


if __name__ == "__main__":
    unittest.main()
