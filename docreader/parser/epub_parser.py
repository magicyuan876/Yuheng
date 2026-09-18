"""EPUB parser.

Parses EPUB files into markdown text and optional embedded images.

An EPUB is a ZIP container: ``META-INF/container.xml`` points at an OPF package
document, whose ``<manifest>`` lists every resource and whose ``<spine>`` gives
the reading order. This parser reads those two files directly with the standard
library, so it needs no EPUB-specific dependency. Files that are not valid EPUB
packages but still contain XHTML (a bare ZIP export, a damaged OPF) fall back to
scanning the archive for HTML documents.
"""

import base64
import logging
import os
import posixpath
import re
import uuid
import zipfile
import xml.etree.ElementTree as ET
from io import BytesIO
from typing import Dict, List, Optional, Tuple
from urllib.parse import unquote

from bs4 import BeautifulSoup

from docreader.models.document import Document
from docreader.parser.base_parser import BaseParser

logger = logging.getLogger(__name__)

_HTML_EXTS = (".html", ".xhtml", ".htm")
_IMAGE_EXTS = (".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg", ".bmp")
_DC_FIELDS = {
    "title": "title",
    "creator": "author",
    "publisher": "publisher",
    "language": "language",
    "description": "description",
    "date": "date",
    "identifier": "isbn",
}


def _localname(tag: str) -> str:
    """Strip the XML namespace from an element tag."""
    return tag.rsplit("}", 1)[-1] if "}" in tag else tag


class EPUBParser(BaseParser):
    """Parser for EPUB e-book files."""

    def __init__(self, *args, extract_images: bool = True, **kwargs):
        super().__init__(*args, **kwargs)
        self.extract_images = extract_images

    # ------------------------------------------------------------------ entry
    def parse_into_text(self, content: bytes) -> Document:
        logger.info(
            "Parsing EPUB file: %s, size: %d bytes", self.file_name, len(content)
        )
        metadata: Dict[str, object] = {"source_format": "epub", "file_size": len(content)}
        images: Dict[str, str] = {}
        image_aliases: Dict[str, str] = {}
        markdown_parts: List[str] = []

        with zipfile.ZipFile(BytesIO(content), "r") as epub_zip:
            names = set(epub_zip.namelist())
            documents, image_files, package_meta = self._read_package(epub_zip, names)
            metadata.update(package_meta)

            if not documents:
                logger.warning(
                    "EPUB %s has no usable OPF spine; scanning the archive for HTML",
                    self.file_name,
                )
                documents = self._scan_for_documents(names)
            if not image_files:
                image_files = [n for n in names if n.lower().endswith(_IMAGE_EXTS)]

            if self.extract_images:
                for img_file in image_files:
                    try:
                        img_data = epub_zip.read(img_file)
                    except KeyError:
                        continue
                    except Exception as e:  # pragma: no cover - corrupt member
                        logger.warning("Failed to extract image %s: %s", img_file, e)
                        continue
                    ext = os.path.splitext(img_file)[1]
                    img_path = f"images/{uuid.uuid4().hex}{ext}"
                    images[img_path] = base64.b64encode(img_data).decode("utf-8")
                    self._add_image_aliases(image_aliases, img_file, img_path)

            for index, html_file in enumerate(documents):
                try:
                    html_content = epub_zip.read(html_file).decode(
                        "utf-8", errors="ignore"
                    )
                except KeyError:
                    logger.warning("EPUB spine references missing file %s", html_file)
                    continue
                chapter_md = self._process_chapter(
                    html_file, html_content, index, image_aliases
                )
                if chapter_md.strip():
                    markdown_parts.append(chapter_md)

        metadata["chapter_count"] = len(markdown_parts)
        metadata["image_count"] = len(images)
        return Document(
            content="\n\n".join(markdown_parts), images=images, metadata=metadata
        )

    # ---------------------------------------------------------------- package
    def _read_package(
        self, epub_zip: zipfile.ZipFile, names: set
    ) -> Tuple[List[str], List[str], Dict[str, str]]:
        """Return (spine documents, image files, Dublin Core metadata).

        Every list is empty when the archive has no readable OPF so the caller
        can fall back to a plain scan.
        """
        opf_path = self._find_opf(epub_zip, names)
        if not opf_path:
            return [], [], {}
        try:
            root = ET.fromstring(epub_zip.read(opf_path))
        except (KeyError, ET.ParseError) as e:
            logger.warning("EPUB OPF %s is unreadable: %s", opf_path, e)
            return [], [], {}

        opf_dir = posixpath.dirname(opf_path)

        def resolve(href: str) -> str:
            href = unquote(href).split("#", 1)[0]
            return self._normalize_epub_path(
                posixpath.join(opf_dir, href) if opf_dir else href
            )

        manifest: Dict[str, Tuple[str, str, str]] = {}
        for item in root.iter():
            if _localname(item.tag) != "item":
                continue
            item_id = item.get("id")
            href = item.get("href")
            if not item_id or not href:
                continue
            manifest[item_id] = (
                resolve(href),
                (item.get("media-type") or "").lower(),
                item.get("properties") or "",
            )

        documents: List[str] = []
        for itemref in root.iter():
            if _localname(itemref.tag) != "itemref":
                continue
            if (itemref.get("linear") or "yes").lower() == "no":
                continue
            entry = manifest.get(itemref.get("idref") or "")
            if not entry:
                continue
            path, media_type, properties = entry
            if "nav" in properties.split():
                continue  # the EPUB 3 table of contents, not book content
            is_document = media_type in ("application/xhtml+xml", "text/html") or (
                not media_type and path.lower().endswith(_HTML_EXTS)
            )
            if is_document and path in names and path not in documents:
                documents.append(path)

        image_files = [
            path
            for path, media_type, _ in manifest.values()
            if path in names
            and (media_type.startswith("image/") or path.lower().endswith(_IMAGE_EXTS))
        ]

        metadata: Dict[str, str] = {}
        authors: List[str] = []
        for node in root.iter():
            name = _localname(node.tag)
            if name not in _DC_FIELDS or not (node.text and node.text.strip()):
                continue
            value = node.text.strip()
            if name == "creator":
                authors.append(value)
            else:
                metadata.setdefault(_DC_FIELDS[name], value)
        if authors:
            metadata["author"] = ", ".join(authors)

        return documents, image_files, metadata

    @staticmethod
    def _find_opf(epub_zip: zipfile.ZipFile, names: set) -> Optional[str]:
        """Locate the OPF via META-INF/container.xml, else by extension."""
        if "META-INF/container.xml" in names:
            try:
                container = ET.fromstring(epub_zip.read("META-INF/container.xml"))
                for rootfile in container.iter():
                    if _localname(rootfile.tag) == "rootfile":
                        full_path = rootfile.get("full-path")
                        if full_path and full_path in names:
                            return full_path
            except ET.ParseError as e:
                logger.warning("EPUB container.xml is unreadable: %s", e)
        candidates = sorted(n for n in names if n.lower().endswith(".opf"))
        return candidates[0] if candidates else None

    @staticmethod
    def _scan_for_documents(names: set) -> List[str]:
        """Fallback ordering for archives without a usable spine."""

        def sort_key(filename: str) -> Tuple[int, str]:
            match = re.search(r"chapter[_-]?(\d+)", filename, re.IGNORECASE)
            return (int(match.group(1)) if match else 999999, filename)

        return sorted(
            (n for n in names if n.lower().endswith(_HTML_EXTS)), key=sort_key
        )

    # ---------------------------------------------------------------- chapters
    def _process_chapter(
        self,
        html_file: str,
        html_content: str,
        index: int,
        image_aliases: Dict[str, str],
    ) -> str:
        try:
            soup = BeautifulSoup(html_content, "lxml")
            title_tag = soup.find(["h1", "h2"])
            if title_tag and title_tag.get_text().strip():
                chapter_title = title_tag.get_text().strip()
                title_tag.decompose()
            else:
                chapter_title = self._title_from_filename(html_file)
            body_html = str(soup.body) if soup.body else html_content
            chapter_md = self._html_to_markdown(
                body_html,
                image_aliases=image_aliases,
                base_path=posixpath.dirname(html_file),
            )
            if not chapter_md.strip():
                return ""
            return f"## {chapter_title}\n\n{chapter_md}"
        except Exception as e:
            logger.error("Failed to process chapter %s: %s", html_file, e)
            return f"## Chapter {index + 1}\n\n[Error processing chapter: {e}]"

    @staticmethod
    def _title_from_filename(html_file: str) -> str:
        base = os.path.splitext(os.path.basename(html_file))[0]
        base = re.sub(r"chapter[_-]?", "Chapter ", base, flags=re.IGNORECASE)
        return base.replace("_", " ").replace("-", " ").strip().title() or "Chapter"

    def _html_to_markdown(
        self,
        html_content: str,
        image_aliases: Dict[str, str] | None = None,
        base_path: str = "",
    ) -> str:
        try:
            from bs4 import Comment
            from markdownify import markdownify as md

            soup = BeautifulSoup(html_content, "lxml")
            for element in soup(["script", "style"]):
                element.decompose()
            for comment in soup.find_all(
                string=lambda text: isinstance(text, Comment)
            ):
                comment.extract()
            self._strip_internal_links(soup)
            if image_aliases:
                self._rewrite_image_sources(soup, image_aliases, base_path)
            markdown_text = md(str(soup), heading_style="ATX")
            return "\n".join(
                line.strip() for line in markdown_text.split("\n") if line.strip()
            )
        except ImportError:
            logger.warning("markdownify not available, using HTML as-is")
            return f"```html\n{html_content}\n```"
        except Exception as e:
            logger.error("HTML to Markdown conversion failed: %s", e)
            return f"```html\n{html_content}\n```"

    # ----------------------------------------------------------------- helpers
    @staticmethod
    def _strip_internal_links(soup: BeautifulSoup) -> None:
        """Unwrap links that don't point to an external resource.

        EPUB internal links (other chapter files, ``#fragment`` anchors, TOC
        entries) become dead links after extraction. Keep only external links
        and replace everything else with its text.
        """
        external = ("http://", "https://", "mailto:", "tel:")
        for link in soup.find_all("a"):
            href = (link.get("href") or "").strip().lower()
            if not href or not href.startswith(external):
                link.unwrap()

    @staticmethod
    def _add_image_aliases(
        image_aliases: Dict[str, str],
        original_path: str,
        image_path: str,
    ) -> None:
        normalized = EPUBParser._normalize_epub_path(original_path)
        aliases = {
            original_path,
            normalized,
            unquote(original_path),
            unquote(normalized),
            posixpath.basename(normalized),
        }
        for alias in aliases:
            if alias:
                image_aliases[alias] = image_path

    @staticmethod
    def _rewrite_image_sources(
        soup: BeautifulSoup,
        image_aliases: Dict[str, str],
        base_path: str = "",
    ) -> None:
        for img in soup.find_all("img"):
            src = (img.get("src") or "").strip()
            if not src:
                continue
            normalized_src = EPUBParser._normalize_epub_path(src)
            candidates = [
                src,
                normalized_src,
                unquote(src),
                unquote(normalized_src),
                posixpath.basename(normalized_src),
            ]
            if base_path:
                joined = EPUBParser._normalize_epub_path(posixpath.join(base_path, src))
                candidates.extend([joined, unquote(joined)])
            for candidate in candidates:
                if candidate in image_aliases:
                    img["src"] = image_aliases[candidate]
                    break

    @staticmethod
    def _normalize_epub_path(path: str) -> str:
        path = unquote(path).split("#", 1)[0].split("?", 1)[0].replace("\\", "/")
        normalized = posixpath.normpath(path)
        return "" if normalized == "." else normalized.lstrip("/")
