"""Video parser: extracts the audio track and timestamped keyframes.

The heavy media work (ffmpeg demux / scene detection) happens here in the
sidecar; model calls stay in the Go app, mirroring the audio split:
  - the extracted audio track is returned via Document.audio_segments and
    transcribed by the Go-side ASR model with segment timestamps;
  - keyframes are returned via Document.images with their millisecond
    positions in Document.image_timestamps, and captioned by the Go-side
    VLM multimodal pipeline.

Frame selection runs ffmpeg scene detection first, then falls back to a
fixed interval when the video has too few scene changes (e.g. a static
lecture recording), so every video yields a usable visual timeline.
"""

import base64
import json
import logging
import os
import re
import shutil
import subprocess
import tempfile
from dataclasses import dataclass
from typing import List, Optional, Tuple

from docreader.config import CONFIG
from docreader.models.document import AudioSegment, Document
from docreader.parser.base_parser import BaseParser

logger = logging.getLogger(__name__)

VIDEO_FILE_TYPES = ("mp4", "mov", "avi", "mkv", "webm", "wmv", "flv", "m4v")

# ref_path prefix for extracted keyframes; the frame timestamp also travels
# in the filename (vframe_<ms>.jpg) as a defensive fallback for transports
# that lose Document.image_timestamps.
_FRAME_REF_DIR = "video_frames"
_FRAME_NAME_RE = re.compile(r"vframe_(\d+)\.jpg$")

_SHOWINFO_PTS_RE = re.compile(r"pts_time:(\d+(?:\.\d+)?)")


def frame_timestamp_from_ref(ref_path: str) -> Optional[int]:
    """Recover the millisecond timestamp encoded in a frame ref path."""
    m = _FRAME_NAME_RE.search(ref_path)
    if not m:
        return None
    return int(m.group(1))


def ffmpeg_available() -> Tuple[bool, str]:
    """Report whether the ffmpeg + ffprobe binaries are on PATH."""
    for binary in ("ffmpeg", "ffprobe"):
        if shutil.which(binary) is None:
            return False, f"{binary} not found on PATH"
    return True, ""


@dataclass
class VideoProbe:
    duration_ms: int
    width: int
    height: int
    has_audio: bool


class VideoParser(BaseParser):
    """Parser for video files (mp4/mov/avi/mkv/webm/wmv/flv/m4v).

    Returns a placeholder markdown body — the Go app rebuilds the real
    timeline markdown from the ASR transcript and these frames.
    """

    def __init__(
        self,
        file_name: str = "",
        file_type: Optional[str] = None,
        **kwargs,
    ):
        super().__init__(file_name=file_name, file_type=file_type, **kwargs)
        # Per-request overrides arrive as parser kwargs (strings from the
        # gRPC ReadConfig.parser_engine_overrides map); fall back to env config.
        self.scene_threshold = self._float_opt(
            kwargs.get("video_scene_threshold"), CONFIG.video_scene_threshold
        )
        self.min_frame_interval_sec = self._int_opt(
            kwargs.get("video_min_frame_interval_sec"),
            CONFIG.video_min_frame_interval_sec,
        )
        self.max_frames = self._int_opt(
            kwargs.get("video_max_frames"), CONFIG.video_max_frames
        )
        self.max_duration_sec = self._int_opt(
            kwargs.get("video_max_duration_sec"), CONFIG.video_max_duration_sec
        )
        self.frame_max_edge = CONFIG.video_frame_max_edge
        self.ffmpeg_timeout = CONFIG.video_ffmpeg_timeout_sec
        self.audio_bitrate = CONFIG.video_audio_bitrate
        self.audio_segment_sec = self._int_opt(
            kwargs.get("video_audio_segment_sec"), CONFIG.video_audio_segment_sec
        )

    @staticmethod
    def _int_opt(value, default: int) -> int:
        try:
            v = int(str(value).strip())
            return v if v >= 0 else default
        except Exception:
            return default

    @staticmethod
    def _float_opt(value, default: float) -> float:
        try:
            v = float(str(value).strip())
            return v if v > 0 else default
        except Exception:
            return default

    def parse_into_text(self, content: bytes) -> Document:
        suffix = f".{self.file_type or 'mp4'}"
        with tempfile.TemporaryDirectory(prefix="docreader_video_") as workdir:
            src_path = os.path.join(workdir, f"input{suffix}")
            with open(src_path, "wb") as f:
                f.write(content)
            return self.parse_from_path(src_path)

    def parse_from_path(self, src_path: str) -> Document:
        """Parse a video directly from a filesystem path.

        Used for shared-volume reads of large videos: the file never travels
        through gRPC message memory — ffmpeg reads it in place.
        """
        ok, reason = ffmpeg_available()
        if not ok:
            raise RuntimeError(
                f"video parsing requires ffmpeg in the docreader image: {reason}"
            )

        with tempfile.TemporaryDirectory(prefix="docreader_video_") as workdir:
            probe = self._probe(src_path)
            logger.info(
                "Video probe: file=%s duration_ms=%d size=%dx%d has_audio=%s",
                self.file_name,
                probe.duration_ms,
                probe.width,
                probe.height,
                probe.has_audio,
            )

            if (
                self.max_duration_sec > 0
                and probe.duration_ms > self.max_duration_sec * 1000
            ):
                raise RuntimeError(
                    f"video duration {probe.duration_ms // 1000}s exceeds the "
                    f"allowed maximum of {self.max_duration_sec}s"
                )

            audio_segments = []
            if probe.has_audio:
                audio_segments = self._extract_audio_segments(
                    src_path, workdir, probe.duration_ms
                )

            frames = self._extract_frames(src_path, workdir, probe)

            images = {}
            timestamps = {}
            for ts_ms, frame_path in frames:
                ref_path = f"{_FRAME_REF_DIR}/vframe_{ts_ms}.jpg"
                with open(frame_path, "rb") as f:
                    images[ref_path] = base64.b64encode(f.read()).decode()
                timestamps[ref_path] = ts_ms

            metadata = {
                "is_video": "true",
                "duration_ms": str(probe.duration_ms),
                "width": str(probe.width),
                "height": str(probe.height),
                "has_audio": "true" if audio_segments else "false",
                "frame_count": str(len(frames)),
            }

            return Document(
                content=f"[Video: {self.file_name}]",
                images=images,
                image_timestamps=timestamps,
                audio_segments=audio_segments,
                audio_mime="audio/mpeg" if audio_segments else "",
                metadata=metadata,
            )

    # ---- ffmpeg helpers ----

    def _run(self, cmd: List[str], what: str) -> subprocess.CompletedProcess:
        logger.info("Running %s: %s", what, " ".join(cmd))
        try:
            proc = subprocess.run(
                cmd,
                capture_output=True,
                timeout=self.ffmpeg_timeout,
                check=False,
            )
        except subprocess.TimeoutExpired as e:
            raise RuntimeError(f"{what} timed out after {self.ffmpeg_timeout}s") from e
        if proc.returncode != 0:
            stderr_tail = (proc.stderr or b"")[-2000:].decode("utf-8", "replace")
            raise RuntimeError(f"{what} failed (rc={proc.returncode}): {stderr_tail}")
        return proc

    def _probe(self, src_path: str) -> VideoProbe:
        proc = self._run(
            [
                "ffprobe",
                "-v",
                "error",
                "-print_format",
                "json",
                "-show_format",
                "-show_streams",
                src_path,
            ],
            "ffprobe",
        )
        try:
            info = json.loads(proc.stdout.decode("utf-8", "replace"))
        except json.JSONDecodeError as e:
            raise RuntimeError(f"ffprobe returned invalid JSON: {e}") from e

        duration_s = 0.0
        try:
            duration_s = float(info.get("format", {}).get("duration") or 0)
        except (TypeError, ValueError):
            duration_s = 0.0

        width = height = 0
        has_audio = False
        has_video = False
        for stream in info.get("streams", []):
            codec_type = stream.get("codec_type")
            if codec_type == "video" and not width:
                has_video = True
                width = int(stream.get("width") or 0)
                height = int(stream.get("height") or 0)
                if duration_s <= 0:
                    try:
                        duration_s = float(stream.get("duration") or 0)
                    except (TypeError, ValueError):
                        pass
            elif codec_type == "audio":
                has_audio = True

        if not has_video:
            raise RuntimeError("file contains no video stream")

        return VideoProbe(
            duration_ms=int(duration_s * 1000),
            width=width,
            height=height,
            has_audio=has_audio,
        )

    def _extract_audio_segments(
        self, src_path: str, workdir: str, duration_ms: int
    ) -> List[AudioSegment]:
        """Extract the audio track as fixed-duration 16kHz mono MP3 segments.

        Segment-wise output keeps each downstream ASR request under provider
        payload caps (DashScope base64 ≤10MB, OpenAI ≤25MB) and provides a
        segment-level timeline for providers that return no timestamps.
        """
        seg_sec = max(self.audio_segment_sec, 30)
        out_pattern = os.path.join(workdir, "audio_%06d.mp3")
        self._run(
            [
                "ffmpeg",
                "-hide_banner",
                "-nostdin",
                "-i",
                src_path,
                "-vn",
                "-ac",
                "1",
                "-ar",
                "16000",
                "-b:a",
                self.audio_bitrate,
                "-f",
                "segment",
                "-segment_time",
                str(seg_sec),
                "-y",
                out_pattern,
            ],
            "ffmpeg audio extraction",
        )

        segments: List[AudioSegment] = []
        i = 0
        total_bytes = 0
        while True:
            seg_path = os.path.join(workdir, f"audio_{i:06d}.mp3")
            if not os.path.exists(seg_path):
                break
            with open(seg_path, "rb") as f:
                data = f.read()
            start_ms = i * seg_sec * 1000
            end_ms = min(start_ms + seg_sec * 1000, duration_ms) if duration_ms else (
                start_ms + seg_sec * 1000
            )
            segments.append(AudioSegment(start_ms=start_ms, end_ms=end_ms, data=data))
            total_bytes += len(data)
            i += 1

        logger.info(
            "Extracted audio track: %d segments, %d bytes total (segment=%ds)",
            len(segments),
            total_bytes,
            seg_sec,
        )
        return segments

    def _scale_filter(self) -> str:
        # Cap the long edge while preserving aspect ratio and never upscale;
        # force_divisible_by keeps dimensions even as encoders require. The
        # escaped commas keep min() intact inside the filtergraph. The final
        # format=yuvj420p converts to full-range YUV — modern ffmpeg's mjpeg
        # encoder refuses limited-range input outright.
        edge = self.frame_max_edge
        return (
            f"scale=min({edge}\\,iw):min({edge}\\,ih):"
            "force_original_aspect_ratio=decrease:force_divisible_by=2,"
            "format=yuvj420p"
        )

    def _extract_frames(
        self, src_path: str, workdir: str, probe: VideoProbe
    ) -> List[Tuple[int, str]]:
        """Extract keyframes as (timestamp_ms, file_path), ordered by time."""
        frames = self._extract_scene_frames(src_path, workdir)

        # Static-camera videos produce almost no scene changes; back-fill with
        # a fixed-interval pass so long lectures still get a visual timeline.
        duration_s = probe.duration_ms / 1000.0
        min_expected = max(1, int(duration_s / 120)) if duration_s > 0 else 1
        if len(frames) < min_expected:
            logger.info(
                "Scene detection yielded %d frames (<%d expected), "
                "falling back to interval sampling",
                len(frames),
                min_expected,
            )
            frames = self._extract_interval_frames(src_path, workdir, duration_s)

        frames = self._enforce_min_interval(frames)
        frames = self._downsample(frames)
        return frames

    def _extract_scene_frames(
        self, src_path: str, workdir: str
    ) -> List[Tuple[int, str]]:
        out_pattern = os.path.join(workdir, "scene_%06d.jpg")
        vf = (
            f"select='gt(scene,{self.scene_threshold})',"
            f"{self._scale_filter()},showinfo"
        )
        proc = self._run(
            [
                "ffmpeg",
                "-hide_banner",
                "-nostdin",
                "-i",
                src_path,
                "-vf",
                vf,
                "-fps_mode",
                "vfr",
                "-q:v",
                "4",
                "-y",
                out_pattern,
            ],
            "ffmpeg scene detection",
        )
        # showinfo logs one "pts_time:<sec>" line per selected frame, in output
        # order, matching scene_000001.jpg, scene_000002.jpg, ...
        stderr = (proc.stderr or b"").decode("utf-8", "replace")
        timestamps = [
            int(float(m.group(1)) * 1000) for m in _SHOWINFO_PTS_RE.finditer(stderr)
        ]

        frames: List[Tuple[int, str]] = []
        for i, ts_ms in enumerate(timestamps, start=1):
            path = os.path.join(workdir, f"scene_{i:06d}.jpg")
            if os.path.exists(path):
                frames.append((ts_ms, path))
        return frames

    def _extract_interval_frames(
        self, src_path: str, workdir: str, duration_s: float
    ) -> List[Tuple[int, str]]:
        interval = max(self.min_frame_interval_sec, 1)
        if duration_s > 0:
            # Aim for roughly one frame per minute (short videos still get at
            # least one frame), bounded below by the configured minimum
            # interval and above by the frame cap.
            target_frames = min(max(self.max_frames, 1), max(1, int(duration_s / 60)))
            interval = max(interval, int(duration_s / target_frames))
        out_pattern = os.path.join(workdir, "interval_%06d.jpg")
        self._run(
            [
                "ffmpeg",
                "-hide_banner",
                "-nostdin",
                "-i",
                src_path,
                "-vf",
                f"fps=1/{interval},{self._scale_filter()}",
                "-q:v",
                "4",
                "-y",
                out_pattern,
            ],
            "ffmpeg interval sampling",
        )
        frames: List[Tuple[int, str]] = []
        i = 1
        while True:
            path = os.path.join(workdir, f"interval_{i:06d}.jpg")
            if not os.path.exists(path):
                break
            # fps=1/N emits the first frame at t≈0 then one every N seconds.
            frames.append(((i - 1) * interval * 1000, path))
            i += 1
        return frames

    def _enforce_min_interval(
        self, frames: List[Tuple[int, str]]
    ) -> List[Tuple[int, str]]:
        min_gap_ms = self.min_frame_interval_sec * 1000
        if min_gap_ms <= 0:
            return frames
        kept: List[Tuple[int, str]] = []
        last_ts = -min_gap_ms
        for ts_ms, path in frames:
            if ts_ms - last_ts >= min_gap_ms:
                kept.append((ts_ms, path))
                last_ts = ts_ms
        return kept

    def _downsample(self, frames: List[Tuple[int, str]]) -> List[Tuple[int, str]]:
        if self.max_frames <= 0 or len(frames) <= self.max_frames:
            return frames
        # Uniformly sample to the cap, always keeping the first frame.
        step = len(frames) / self.max_frames
        kept = [frames[min(int(i * step), len(frames) - 1)] for i in range(self.max_frames)]
        # De-duplicate potential rounding collisions while preserving order.
        seen = set()
        result = []
        for item in kept:
            if item[0] not in seen:
                seen.add(item[0])
                result.append(item)
        logger.info("Downsampled %d frames to %d (cap)", len(frames), len(result))
        return result
