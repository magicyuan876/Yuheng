"""Chunk document schema."""

import json
from typing import Any, Dict, List, Optional

from pydantic import BaseModel, Field


class Chunk(BaseModel):
    """Document Chunk including chunk content, chunk metadata."""

    content: str = Field(default="", description="chunk text content")
    seq: int = Field(default=0, description="Chunk sequence number")
    start: int = Field(default=0, description="Chunk start position")
    end: int = Field(description="Chunk end position")
    images: List[Dict[str, Any]] = Field(
        default_factory=list, description="Images in the chunk"
    )

    metadata: Dict[str, Any] = Field(
        default_factory=dict,
        description="metadata fields",
    )

    def to_dict(self, **kwargs: Any) -> Dict[str, Any]:
        """Convert Chunk to dict."""

        data = self.model_dump()
        data.update(kwargs)
        data["class_name"] = self.__class__.__name__
        return data

    def to_json(self, **kwargs: Any) -> str:
        """Convert Chunk to json."""
        data = self.to_dict(**kwargs)
        return json.dumps(data)

    def __hash__(self):
        """Hash function."""
        return hash((self.content,))

    def __eq__(self, other):
        """Equal function."""
        return self.content == other.content

    @classmethod
    def from_dict(cls, data: Dict[str, Any], **kwargs: Any):  # type: ignore
        """Create Chunk from dict."""
        if isinstance(kwargs, dict):
            data.update(kwargs)

        data.pop("class_name", None)
        return cls(**data)

    @classmethod
    def from_json(cls, data_str: str, **kwargs: Any):  # type: ignore
        """Create Chunk from json."""
        data = json.loads(data_str)
        return cls.from_dict(data, **kwargs)


class Document(BaseModel):
    """Document including document content, document metadata."""

    model_config = {"arbitrary_types_allowed": True}

    content: str = Field(default="", description="document text content")
    images: Dict[str, str] = Field(
        default_factory=dict, description="Images in the document"
    )

    # Media fields (populated by VideoParser only).
    # audio_segments carries the extracted audio track for Go-side ASR, split
    # into fixed-duration pieces so providers with small payload caps and no
    # timestamp support still yield a usable timeline. audio_bytes is the
    # legacy single-blob form (still honoured by the transport).
    # image_timestamps maps an images{} ref_path to its position in the
    # source media, in milliseconds.
    audio_segments: List["AudioSegment"] = Field(
        default_factory=list, description="Extracted audio track segments for ASR"
    )
    audio_bytes: Optional[bytes] = Field(
        default=None, description="Extracted audio track for ASR (single blob)"
    )
    audio_mime: str = Field(default="", description="MIME type of the audio track")
    image_timestamps: Dict[str, int] = Field(
        default_factory=dict, description="ref_path -> timestamp_ms for video frames"
    )

    chunks: List[Chunk] = Field(default_factory=list, description="document chunks")
    metadata: Dict[str, Any] = Field(
        default_factory=dict,
        description="metadata fields",
    )

    def has_audio(self) -> bool:
        return bool(self.audio_segments) or bool(self.audio_bytes)

    def set_content(self, content: str) -> None:
        """Set document content."""
        self.content = content

    def get_content(self) -> str:
        """Get document content."""
        return self.content

    def is_valid(self) -> bool:
        return self.content != ""


class AudioSegment(BaseModel):
    """One fixed-duration piece of an extracted audio track."""

    model_config = {"arbitrary_types_allowed": True}

    start_ms: int = Field(default=0)
    end_ms: int = Field(default=0)
    data: bytes = Field(default=b"")


Document.model_rebuild()
