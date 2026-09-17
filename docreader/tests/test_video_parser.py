import subprocess
import tempfile
import unittest

from docreader.parser.video_parser import (
    VIDEO_FILE_TYPES,
    VideoParser,
    ffmpeg_available,
    frame_timestamp_from_ref,
)

_FFMPEG_OK, _FFMPEG_REASON = ffmpeg_available()


def _synthetic_video_bytes(duration_sec: int = 4, with_audio: bool = True) -> bytes:
    """Generate a tiny test video with ffmpeg (testsrc + optional sine audio)."""
    with tempfile.NamedTemporaryFile(suffix=".mp4", delete=False) as f:
        out_path = f.name
    cmd = [
        "ffmpeg",
        "-hide_banner",
        "-nostdin",
        "-f",
        "lavfi",
        "-i",
        f"testsrc=duration={duration_sec}:size=320x240:rate=10",
    ]
    if with_audio:
        cmd += ["-f", "lavfi", "-i", f"sine=frequency=440:duration={duration_sec}"]
    cmd += ["-pix_fmt", "yuv420p", "-y", out_path]
    subprocess.run(cmd, capture_output=True, check=True)
    with open(out_path, "rb") as f:
        return f.read()


class TestFrameTimestampFromRef(unittest.TestCase):
    def test_round_trip(self):
        self.assertEqual(
            frame_timestamp_from_ref("video_frames/vframe_12500.jpg"), 12500
        )
        self.assertEqual(frame_timestamp_from_ref("video_frames/vframe_0.jpg"), 0)

    def test_non_frame_paths(self):
        self.assertIsNone(frame_timestamp_from_ref("images/pic.png"))
        self.assertIsNone(frame_timestamp_from_ref("video_frames/other.jpg"))


class TestFrameSelectionLogic(unittest.TestCase):
    """Pure-logic tests for interval enforcement and downsampling (no ffmpeg)."""

    def _parser(self, **overrides) -> VideoParser:
        return VideoParser(file_name="test.mp4", **overrides)

    def test_enforce_min_interval(self):
        p = self._parser(video_min_frame_interval_sec="5")
        frames = [(0, "a"), (2000, "b"), (5000, "c"), (7000, "d"), (12000, "e")]
        kept = p._enforce_min_interval(frames)
        self.assertEqual([ts for ts, _ in kept], [0, 5000, 12000])

    def test_downsample_respects_cap_and_keeps_first(self):
        p = self._parser(video_max_frames="3")
        frames = [(i * 1000, str(i)) for i in range(10)]
        kept = p._downsample(frames)
        self.assertLessEqual(len(kept), 3)
        self.assertEqual(kept[0][0], 0)
        # Order preserved
        self.assertEqual([ts for ts, _ in kept], sorted(ts for ts, _ in kept))

    def test_downsample_noop_under_cap(self):
        p = self._parser(video_max_frames="200")
        frames = [(0, "a"), (1000, "b")]
        self.assertEqual(p._downsample(frames), frames)

    def test_override_parsing_falls_back_on_garbage(self):
        p = self._parser(
            video_max_frames="not-a-number",
            video_scene_threshold="-1",
            video_min_frame_interval_sec="7",
        )
        self.assertEqual(p.min_frame_interval_sec, 7)
        # Garbage falls back to config defaults
        self.assertGreater(p.max_frames, 0)
        self.assertGreater(p.scene_threshold, 0)


@unittest.skipUnless(_FFMPEG_OK, f"ffmpeg unavailable: {_FFMPEG_REASON}")
class TestVideoParserWithFfmpeg(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.video_bytes = _synthetic_video_bytes(duration_sec=4, with_audio=True)

    def test_parse_extracts_audio_and_frames(self):
        parser = VideoParser(
            file_name="test.mp4",
            video_min_frame_interval_sec="1",
            video_max_frames="10",
        )
        doc = parser.parse(self.video_bytes)

        self.assertTrue(doc.content.startswith("[Video:"))
        self.assertEqual(doc.metadata["is_video"], "true")
        self.assertGreater(int(doc.metadata["duration_ms"]), 3000)
        self.assertEqual(doc.metadata["has_audio"], "true")

        self.assertTrue(doc.audio_segments)
        self.assertTrue(doc.audio_segments[0].data)
        self.assertEqual(doc.audio_segments[0].start_ms, 0)
        self.assertGreater(doc.audio_segments[0].end_ms, 0)
        self.assertEqual(doc.audio_mime, "audio/mpeg")

        self.assertGreater(len(doc.images), 0)
        self.assertEqual(set(doc.images.keys()), set(doc.image_timestamps.keys()))
        for ref_path, ts in doc.image_timestamps.items():
            self.assertEqual(frame_timestamp_from_ref(ref_path), ts)

    def test_parse_without_audio_track(self):
        video = _synthetic_video_bytes(duration_sec=2, with_audio=False)
        parser = VideoParser(file_name="silent.mp4", video_min_frame_interval_sec="1")
        doc = parser.parse(video)
        self.assertEqual(doc.metadata["has_audio"], "false")
        self.assertFalse(doc.has_audio())
        self.assertGreater(len(doc.images), 0)

    def test_duration_cap_rejects_long_video(self):
        parser = VideoParser(file_name="test.mp4", video_max_duration_sec="1")
        with self.assertRaises(RuntimeError):
            parser.parse(self.video_bytes)


@unittest.skipUnless(_FFMPEG_OK, f"ffmpeg unavailable: {_FFMPEG_REASON}")
class TestVideoParserFromPath(unittest.TestCase):
    """Shared-volume path mode: ffmpeg reads the file in place."""

    def test_parse_from_path_matches_bytes_mode(self):
        video = _synthetic_video_bytes(duration_sec=2, with_audio=True)
        with tempfile.NamedTemporaryFile(suffix=".mp4", delete=False) as f:
            f.write(video)
            path = f.name

        parser = VideoParser(file_name="ondisk.mp4", video_min_frame_interval_sec="1")
        doc = parser.parse_from_path(path)
        self.assertEqual(doc.metadata["is_video"], "true")
        self.assertTrue(doc.has_audio())
        self.assertGreater(len(doc.images), 0)


class TestSharedPathGuard(unittest.TestCase):
    def test_rejects_paths_outside_shared_dir(self):
        import os

        from docreader.main import DocReaderServicer

        base = tempfile.mkdtemp(prefix="shared_base_")
        inside = os.path.join(base, "ok.mp4")
        with open(inside, "wb") as f:
            f.write(b"x")

        from unittest import mock

        with mock.patch("docreader.main.CONFIG") as cfg:
            cfg.shared_data_dir = base
            resolved = DocReaderServicer._resolve_shared_path(inside)
            self.assertTrue(resolved.endswith("ok.mp4"))
            with self.assertRaises(ValueError):
                DocReaderServicer._resolve_shared_path(
                    os.path.join(base, "..", "escape.mp4")
                )
            with self.assertRaises(FileNotFoundError):
                DocReaderServicer._resolve_shared_path(os.path.join(base, "missing.mp4"))


class TestRegistryRegistration(unittest.TestCase):
    def test_video_types_registered_when_ffmpeg_present(self):
        from docreader.parser.registry import registry

        if not _FFMPEG_OK:
            self.skipTest(f"ffmpeg unavailable: {_FFMPEG_REASON}")
        for ext in VIDEO_FILE_TYPES:
            cls = registry.get_parser_class("", ext)
            self.assertIs(cls, VideoParser, f"extension {ext} should map to VideoParser")


if __name__ == "__main__":
    unittest.main()
