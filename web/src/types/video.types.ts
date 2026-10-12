// src/domain/video.enums.ts
export enum VideoStatus {
  UploadPending = "upload_pending",
  Uploaded = "uploaded",
  Valid = "valid",
  ThumbnailGenerated = "thumbnail_generated",
  Transcoded = "transcoded",
  SegmentGenerated = "segment_generated",
  Checksum = "checksum",
  Published = "published",
  Done = "done",
  Phase1Failed = "phase_1_failed",
  Phase2Failed = "phase_2_failed",
}

export enum VideoVisibility {
  Public = "public",
  Private = "private",
  Unlisted = "unlisted",
}

export interface UploadVideoDto {
  size: number;
  file_name: string;
}

export interface VideoConfirmDto {
  thumbnail: string;
  title: string;
  description?: string;
  visibility: VideoVisibility;
}
export interface PresignedResult {
  key: string;
  url: string;
  id: string;
}

export interface Video {
  id: string;
  user_id: string;
  file_key: string;
  thumbnail: string;
  title: string;
  description: string;
  visibility: VideoVisibility;
  status: VideoStatus;
  stage: number;
  created_at: string; // ISO string
  updated_at: string; // ISO string
}
