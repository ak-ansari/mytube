import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  STALE_UPLOAD_MS,
  channelName,
  formatCount,
  formatDate,
  formatDuration,
  formatSubscribers,
  formatViews,
  timeAgo,
  videoStatusInfo,
  visibilityIcon,
} from "@/utils/format";
import { getExtension, stripExtension } from "@/utils";
import { VideoStatus } from "@/types";

describe("formatCount", () => {
  it.each([
    [0, "0"],
    [7, "7"],
    [999, "999"],
    [1000, "1K"],
    [1234, "1.2K"],
    [1250, "1.3K"],
    [10_000, "10K"],
    [999_000, "999K"],
    [1_000_000, "1M"],
    [1_500_000, "1.5M"],
    [12_340_000, "12.3M"],
    [1_000_000_000, "1B"],
    [2_560_000_000, "2.6B"],
  ])("%d -> %s", (n, expected) => {
    expect(formatCount(n)).toBe(expected);
  });

  // 999_999 / 1000 = 999.999 -> toFixed(1) = "1000.0" -> "1000K" instead of rolling over to "1M"
  it("rolls values that round up to 1000 into the next unit", () => {
    expect(formatCount(999_949)).toBe("999.9K");
    expect(formatCount(999_950)).toBe("1M");
    expect(formatCount(999_999)).toBe("1M");
    expect(formatCount(999_999_999)).toBe("1B");
    // there is no unit above B, so it keeps counting in B
    expect(formatCount(999_999_999_999)).toBe("1000B");
  });
});

describe("formatViews / formatSubscribers", () => {
  it("pluralises views", () => {
    expect(formatViews(0)).toBe("0 views");
    expect(formatViews(1)).toBe("1 view");
    expect(formatViews(2)).toBe("2 views");
    expect(formatViews(1500)).toBe("1.5K views");
  });
  it("pluralises subscribers", () => {
    expect(formatSubscribers(0)).toBe("0 subscribers");
    expect(formatSubscribers(1)).toBe("1 subscriber");
    expect(formatSubscribers(2_300_000)).toBe("2.3M subscribers");
  });
});

describe("formatDuration", () => {
  it.each([
    [undefined, ""],
    [null, ""],
    [0, ""],
    [-5, ""],
    [5, "0:05"],
    [59.9, "0:59"],
    [60, "1:00"],
    [75, "1:15"],
    [600, "10:00"],
    [3600, "1:00:00"],
    [3725, "1:02:05"],
    [36_061, "10:01:01"],
  ])("%s -> %j", (s, expected) => {
    expect(formatDuration(s as number | null | undefined)).toBe(expected);
  });
});

describe("timeAgo", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-01-10T12:00:00Z"));
  });
  afterEach(() => vi.useRealTimers());

  it("is relative to the current time", () => {
    expect(timeAgo(new Date("2026-01-10T11:59:50Z"))).toBe("a few seconds ago");
    expect(timeAgo("2026-01-10T11:00:00Z")).toBe("an hour ago");
    expect(timeAgo("2026-01-10T09:00:00Z")).toBe("3 hours ago");
    expect(timeAgo("2026-01-08T12:00:00Z")).toBe("2 days ago");
    expect(timeAgo("2025-01-10T12:00:00Z")).toBe("a year ago");
  });

  it("moves with the clock", () => {
    const posted = "2026-01-10T12:00:00Z";
    expect(timeAgo(posted)).toBe("a few seconds ago");
    vi.setSystemTime(new Date("2026-01-10T12:05:00Z"));
    expect(timeAgo(posted)).toBe("5 minutes ago");
  });
});

describe("formatDate", () => {
  it("formats as MMM D, YYYY", () => {
    expect(formatDate("2024-01-15T12:00:00Z")).toBe("Jan 15, 2024");
    expect(formatDate(new Date(2023, 11, 3))).toBe("Dec 3, 2023");
  });
});

describe("channelName", () => {
  it("prefers channel_name and falls back to user_name", () => {
    expect(channelName({ channel_name: "Alice Films", user_name: "alice" })).toBe("Alice Films");
    expect(channelName({ channel_name: null, user_name: "alice" })).toBe("alice");
    expect(channelName({ channel_name: "", user_name: "alice" })).toBe("alice");
  });
});

describe("videoStatusInfo", () => {
  const now = new Date("2026-01-10T12:00:00Z").getTime();
  const at = (msAgo: number) => new Date(now - msAgo).toISOString();
  const v = (status: VideoStatus, extra: Partial<{ title: string | null; created_at: string; updated_at: string }> = {}) => ({
    status,
    title: "Title",
    created_at: at(60_000),
    updated_at: at(60_000),
    ...extra,
  });

  it("published / done", () => {
    for (const s of [VideoStatus.Published, VideoStatus.Done]) {
      expect(videoStatusInfo(v(s), now)).toEqual({
        label: "Published",
        color: "success",
        icon: "mdi-check-circle",
        processing: false,
        needsDetails: false,
      });
    }
  });

  it("failed phases", () => {
    for (const s of [VideoStatus.Phase1Failed, VideoStatus.Phase2Failed]) {
      const info = videoStatusInfo(v(s), now);
      expect(info.label).toBe("Failed");
      expect(info.color).toBe("error");
      expect(info.processing).toBe(false);
    }
  });

  it("fresh upload_pending / uploaded drafts are uploading (processing)", () => {
    for (const s of [VideoStatus.UploadPending, VideoStatus.Uploaded]) {
      const info = videoStatusInfo(v(s, { updated_at: at(STALE_UPLOAD_MS - 60_000) }), now);
      expect(info).toMatchObject({ label: "Uploading", color: "info", processing: true, needsDetails: false });
    }
  });

  it("15-minute stale rule: drafts older than 15 min are 'Upload incomplete' and not processing", () => {
    expect(STALE_UPLOAD_MS).toBe(15 * 60 * 1000);
    for (const s of [VideoStatus.UploadPending, VideoStatus.Uploaded]) {
      const info = videoStatusInfo(v(s, { updated_at: at(STALE_UPLOAD_MS + 1) }), now);
      expect(info).toMatchObject({ label: "Upload incomplete", color: "error", icon: "mdi-cloud-alert", processing: false });
    }
  });

  it("exactly 15 min is not yet stale (strictly greater than)", () => {
    expect(videoStatusInfo(v(VideoStatus.UploadPending, { updated_at: at(STALE_UPLOAD_MS) }), now).label).toBe("Uploading");
  });

  it("falls back to created_at when updated_at is empty", () => {
    expect(videoStatusInfo(v(VideoStatus.UploadPending, { updated_at: "", created_at: at(STALE_UPLOAD_MS * 2) }), now).label).toBe(
      "Upload incomplete"
    );
    expect(videoStatusInfo(v(VideoStatus.UploadPending, { updated_at: "", created_at: at(1000) }), now).label).toBe("Uploading");
  });

  it("uses Date.now() when no clock is passed", () => {
    vi.useFakeTimers();
    vi.setSystemTime(now + STALE_UPLOAD_MS + 5000);
    expect(videoStatusInfo(v(VideoStatus.Uploaded, { updated_at: at(0) })).label).toBe("Upload incomplete");
    vi.useRealTimers();
  });

  it("untitled mid-pipeline videos are drafts that need details", () => {
    for (const s of [VideoStatus.Valid, VideoStatus.ThumbnailGenerated]) {
      expect(videoStatusInfo(v(s, { title: null }), now)).toMatchObject({ label: "Draft", color: "warning", needsDetails: true, processing: false });
      expect(videoStatusInfo(v(s, { title: "" }), now).needsDetails).toBe(true);
    }
  });

  it("titled mid-pipeline videos are processing", () => {
    for (const s of [
      VideoStatus.Valid,
      VideoStatus.ThumbnailGenerated,
      VideoStatus.Transcoded,
      VideoStatus.SegmentGenerated,
      VideoStatus.Checksum,
    ]) {
      expect(videoStatusInfo(v(s), now)).toMatchObject({ label: "Processing", processing: true, needsDetails: false });
    }
  });
});

describe("visibilityIcon", () => {
  it("maps each visibility to an icon", () => {
    expect(visibilityIcon).toEqual({ public: "mdi-earth", unlisted: "mdi-link-variant", private: "mdi-lock-outline" });
    expect(visibilityIcon["unknown"]).toBeUndefined();
  });
});

describe("file name helpers", () => {
  it("getExtension", () => {
    expect(getExtension("clip.mp4")).toBe("mp4");
    expect(getExtension("archive.tar.gz")).toBe("gz");
  });
  it("stripExtension", () => {
    expect(stripExtension("clip.mp4")).toBe("clip");
    expect(stripExtension("my.holiday.mov")).toBe("my.holiday");
    expect(stripExtension("noext")).toBe("noext");
  });
  it("getExtension is empty when there is no extension", () => {
    expect(getExtension("avatar")).toBe("");
    expect(getExtension(".env")).toBe("");
    expect(getExtension("file.")).toBe("");
    expect(getExtension("")).toBe("");
  });
});
