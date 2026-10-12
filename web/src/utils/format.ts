import moment from "moment";
import { VideoStatus, type ChannelSummary, type VideoCard } from "@/types";

const COUNT_UNITS: ReadonlyArray<readonly [number, string]> = [
  [1_000_000_000, "B"],
  [1_000_000, "M"],
  [1_000, "K"],
];

/** 1234 -> "1.2K", 1500000 -> "1.5M", 999999 -> "1M" (not "1000K") */
export const formatCount = (n: number): string => {
  let bigger: readonly [number, string] | undefined;
  for (const unit of COUNT_UNITS) {
    const [size, suffix] = unit;
    if (n >= size) {
      const value = +(n / size).toFixed(1);
      // rounding can reach the next unit (999_999 -> 1000.0K), so show it in that unit instead
      if (value >= 1000 && bigger) return `${+(n / bigger[0]).toFixed(1)}${bigger[1]}`;
      return `${value}${suffix}`;
    }
    bigger = unit;
  }
  return `${n}`;
};

export const formatViews = (n: number) => `${formatCount(n)} ${n === 1 ? "view" : "views"}`;

export const formatSubscribers = (n: number) => `${formatCount(n)} ${n === 1 ? "subscriber" : "subscribers"}`;

/** 75 -> "1:15", 3725 -> "1:02:05" */
export const formatDuration = (seconds?: number | null): string => {
  if (!seconds || seconds < 0) return "";
  const h = Math.floor(seconds / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  const s = Math.floor(seconds % 60);
  const ss = String(s).padStart(2, "0");
  return h > 0 ? `${h}:${String(m).padStart(2, "0")}:${ss}` : `${m}:${ss}`;
};

export const timeAgo = (date: string | Date) => moment(date).fromNow();

export const formatDate = (date: string | Date) => moment(date).format("MMM D, YYYY");

export const channelName = (c: Pick<ChannelSummary, "channel_name" | "user_name">) => c.channel_name || c.user_name;

export interface StatusInfo {
  label: string;
  color: string;
  icon: string;
  /** still moving through the pipeline */
  processing: boolean;
  /** uploaded and checked, waiting for title/visibility (confirm) */
  needsDetails: boolean;
}

/** Drafts still waiting for their file after this long were abandoned (closed tab, failed PUT). */
export const STALE_UPLOAD_MS = 15 * 60 * 1000;

export const videoStatusInfo = (v: Pick<VideoCard, "status" | "title" | "created_at" | "updated_at">, now = Date.now()): StatusInfo => {
  switch (v.status) {
    case VideoStatus.Published:
    case VideoStatus.Done:
      return { label: "Published", color: "success", icon: "mdi-check-circle", processing: false, needsDetails: false };
    case VideoStatus.Phase1Failed:
    case VideoStatus.Phase2Failed:
      return { label: "Failed", color: "error", icon: "mdi-alert-circle", processing: false, needsDetails: false };
    case VideoStatus.UploadPending:
    case VideoStatus.Uploaded:
      if (now - new Date(v.updated_at || v.created_at).getTime() > STALE_UPLOAD_MS) {
        return { label: "Upload incomplete", color: "error", icon: "mdi-cloud-alert", processing: false, needsDetails: false };
      }
      return { label: "Uploading", color: "info", icon: "mdi-cloud-upload", processing: true, needsDetails: false };
  }
  if (!v.title) {
    return { label: "Draft", color: "warning", icon: "mdi-file-edit-outline", processing: false, needsDetails: true };
  }
  return { label: "Processing", color: "info", icon: "mdi-progress-clock", processing: true, needsDetails: false };
};

export const visibilityIcon: Record<string, string> = {
  public: "mdi-earth",
  unlisted: "mdi-link-variant",
  private: "mdi-lock-outline",
};
