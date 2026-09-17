from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class ReadConfig(_message.Message):
    __slots__ = ("parser_engine", "parser_engine_overrides")
    class ParserEngineOverridesEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    PARSER_ENGINE_FIELD_NUMBER: _ClassVar[int]
    PARSER_ENGINE_OVERRIDES_FIELD_NUMBER: _ClassVar[int]
    parser_engine: str
    parser_engine_overrides: _containers.ScalarMap[str, str]
    def __init__(self, parser_engine: _Optional[str] = ..., parser_engine_overrides: _Optional[_Mapping[str, str]] = ...) -> None: ...

class ReadRequest(_message.Message):
    __slots__ = ("file_content", "file_name", "file_type", "url", "title", "config", "request_id", "file_path")
    FILE_CONTENT_FIELD_NUMBER: _ClassVar[int]
    FILE_NAME_FIELD_NUMBER: _ClassVar[int]
    FILE_TYPE_FIELD_NUMBER: _ClassVar[int]
    URL_FIELD_NUMBER: _ClassVar[int]
    TITLE_FIELD_NUMBER: _ClassVar[int]
    CONFIG_FIELD_NUMBER: _ClassVar[int]
    REQUEST_ID_FIELD_NUMBER: _ClassVar[int]
    FILE_PATH_FIELD_NUMBER: _ClassVar[int]
    file_content: bytes
    file_name: str
    file_type: str
    url: str
    title: str
    config: ReadConfig
    request_id: str
    file_path: str
    def __init__(self, file_content: _Optional[bytes] = ..., file_name: _Optional[str] = ..., file_type: _Optional[str] = ..., url: _Optional[str] = ..., title: _Optional[str] = ..., config: _Optional[_Union[ReadConfig, _Mapping]] = ..., request_id: _Optional[str] = ..., file_path: _Optional[str] = ...) -> None: ...

class ImageRef(_message.Message):
    __slots__ = ("filename", "original_ref", "mime_type", "storage_key", "image_data", "timestamp_ms")
    FILENAME_FIELD_NUMBER: _ClassVar[int]
    ORIGINAL_REF_FIELD_NUMBER: _ClassVar[int]
    MIME_TYPE_FIELD_NUMBER: _ClassVar[int]
    STORAGE_KEY_FIELD_NUMBER: _ClassVar[int]
    IMAGE_DATA_FIELD_NUMBER: _ClassVar[int]
    TIMESTAMP_MS_FIELD_NUMBER: _ClassVar[int]
    filename: str
    original_ref: str
    mime_type: str
    storage_key: str
    image_data: bytes
    timestamp_ms: int
    def __init__(self, filename: _Optional[str] = ..., original_ref: _Optional[str] = ..., mime_type: _Optional[str] = ..., storage_key: _Optional[str] = ..., image_data: _Optional[bytes] = ..., timestamp_ms: _Optional[int] = ...) -> None: ...

class MediaInfo(_message.Message):
    __slots__ = ("is_video", "duration_ms", "has_audio", "audio_mime_type", "width", "height")
    IS_VIDEO_FIELD_NUMBER: _ClassVar[int]
    DURATION_MS_FIELD_NUMBER: _ClassVar[int]
    HAS_AUDIO_FIELD_NUMBER: _ClassVar[int]
    AUDIO_MIME_TYPE_FIELD_NUMBER: _ClassVar[int]
    WIDTH_FIELD_NUMBER: _ClassVar[int]
    HEIGHT_FIELD_NUMBER: _ClassVar[int]
    is_video: bool
    duration_ms: int
    has_audio: bool
    audio_mime_type: str
    width: int
    height: int
    def __init__(self, is_video: bool = ..., duration_ms: _Optional[int] = ..., has_audio: bool = ..., audio_mime_type: _Optional[str] = ..., width: _Optional[int] = ..., height: _Optional[int] = ...) -> None: ...

class ReadResponse(_message.Message):
    __slots__ = ("markdown_content", "image_refs", "image_dir_path", "metadata", "error", "media", "audio_data")
    class MetadataEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    MARKDOWN_CONTENT_FIELD_NUMBER: _ClassVar[int]
    IMAGE_REFS_FIELD_NUMBER: _ClassVar[int]
    IMAGE_DIR_PATH_FIELD_NUMBER: _ClassVar[int]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    MEDIA_FIELD_NUMBER: _ClassVar[int]
    AUDIO_DATA_FIELD_NUMBER: _ClassVar[int]
    markdown_content: str
    image_refs: _containers.RepeatedCompositeFieldContainer[ImageRef]
    image_dir_path: str
    metadata: _containers.ScalarMap[str, str]
    error: str
    media: MediaInfo
    audio_data: bytes
    def __init__(self, markdown_content: _Optional[str] = ..., image_refs: _Optional[_Iterable[_Union[ImageRef, _Mapping]]] = ..., image_dir_path: _Optional[str] = ..., metadata: _Optional[_Mapping[str, str]] = ..., error: _Optional[str] = ..., media: _Optional[_Union[MediaInfo, _Mapping]] = ..., audio_data: _Optional[bytes] = ...) -> None: ...

class ReadStreamMeta(_message.Message):
    __slots__ = ("markdown_content", "image_dir_path", "metadata", "error", "image_count", "media")
    class MetadataEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    MARKDOWN_CONTENT_FIELD_NUMBER: _ClassVar[int]
    IMAGE_DIR_PATH_FIELD_NUMBER: _ClassVar[int]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    IMAGE_COUNT_FIELD_NUMBER: _ClassVar[int]
    MEDIA_FIELD_NUMBER: _ClassVar[int]
    markdown_content: str
    image_dir_path: str
    metadata: _containers.ScalarMap[str, str]
    error: str
    image_count: int
    media: MediaInfo
    def __init__(self, markdown_content: _Optional[str] = ..., image_dir_path: _Optional[str] = ..., metadata: _Optional[_Mapping[str, str]] = ..., error: _Optional[str] = ..., image_count: _Optional[int] = ..., media: _Optional[_Union[MediaInfo, _Mapping]] = ...) -> None: ...

class AudioChunk(_message.Message):
    __slots__ = ("data", "segment_index", "start_ms", "end_ms")
    DATA_FIELD_NUMBER: _ClassVar[int]
    SEGMENT_INDEX_FIELD_NUMBER: _ClassVar[int]
    START_MS_FIELD_NUMBER: _ClassVar[int]
    END_MS_FIELD_NUMBER: _ClassVar[int]
    data: bytes
    segment_index: int
    start_ms: int
    end_ms: int
    def __init__(self, data: _Optional[bytes] = ..., segment_index: _Optional[int] = ..., start_ms: _Optional[int] = ..., end_ms: _Optional[int] = ...) -> None: ...

class ReadStreamResponse(_message.Message):
    __slots__ = ("meta", "image", "audio")
    META_FIELD_NUMBER: _ClassVar[int]
    IMAGE_FIELD_NUMBER: _ClassVar[int]
    AUDIO_FIELD_NUMBER: _ClassVar[int]
    meta: ReadStreamMeta
    image: ImageRef
    audio: AudioChunk
    def __init__(self, meta: _Optional[_Union[ReadStreamMeta, _Mapping]] = ..., image: _Optional[_Union[ImageRef, _Mapping]] = ..., audio: _Optional[_Union[AudioChunk, _Mapping]] = ...) -> None: ...

class ListEnginesRequest(_message.Message):
    __slots__ = ("config_overrides",)
    class ConfigOverridesEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    CONFIG_OVERRIDES_FIELD_NUMBER: _ClassVar[int]
    config_overrides: _containers.ScalarMap[str, str]
    def __init__(self, config_overrides: _Optional[_Mapping[str, str]] = ...) -> None: ...

class ParserEngineInfo(_message.Message):
    __slots__ = ("name", "description", "file_types", "available", "unavailable_reason")
    NAME_FIELD_NUMBER: _ClassVar[int]
    DESCRIPTION_FIELD_NUMBER: _ClassVar[int]
    FILE_TYPES_FIELD_NUMBER: _ClassVar[int]
    AVAILABLE_FIELD_NUMBER: _ClassVar[int]
    UNAVAILABLE_REASON_FIELD_NUMBER: _ClassVar[int]
    name: str
    description: str
    file_types: _containers.RepeatedScalarFieldContainer[str]
    available: bool
    unavailable_reason: str
    def __init__(self, name: _Optional[str] = ..., description: _Optional[str] = ..., file_types: _Optional[_Iterable[str]] = ..., available: bool = ..., unavailable_reason: _Optional[str] = ...) -> None: ...

class ListEnginesResponse(_message.Message):
    __slots__ = ("engines",)
    ENGINES_FIELD_NUMBER: _ClassVar[int]
    engines: _containers.RepeatedCompositeFieldContainer[ParserEngineInfo]
    def __init__(self, engines: _Optional[_Iterable[_Union[ParserEngineInfo, _Mapping]]] = ...) -> None: ...
