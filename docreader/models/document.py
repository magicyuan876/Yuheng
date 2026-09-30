"""Parsed document schema returned by every parser."""

from typing import Any, Dict, List

from pydantic import BaseModel, Field


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
    # timestamp support still yield a usable timeline.
    # image_timestamps maps an images{} ref_path to its position in the
    # source media, in milliseconds.
    audio_segments: List["AudioSegment"] = Field(
        default_factory=list, description="Extracted audio track segments for ASR"
    )
    audio_mime: str = Field(default="", description="MIME type of the audio track")
    image_timestamps: Dict[str, int] = Field(
        default_factory=dict, description="ref_path -> timestamp_ms for video frames"
    )

    metadata: Dict[str, Any] = Field(
        default_factory=dict,
        description="metadata fields",
    )

    def has_audio(self) -> bool:
        return bool(self.audio_segments)

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
