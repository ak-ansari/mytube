import type { VideoStatus, VideoVisibility } from "./video.types";

export interface Page<T> {
  items: T[];
  page: number;
  size: number;
  has_more: boolean;
}

export interface ChannelSummary {
  id: string;
  user_name: string;
  channel_name: string | null;
  avatar: string | null; // object key
  avatar_url: string; // presigned url ("" when no avatar)
}

export interface Channel extends ChannelSummary {
  description: string | null;
  subscribers: number;
  video_count: number;
  is_subscribed: boolean;
  created_at: string;
}

export interface Profile extends Channel {
  email: string | null;
}

export interface VideoCard {
  id: string;
  title: string | null;
  description: string | null;
  thumbnail: string | null;
  thumbnail_url: string;
  visibility: VideoVisibility | null;
  status: VideoStatus;
  views: number;
  duration_seconds: number | null;
  created_at: string;
  updated_at: string;
  channel: ChannelSummary;
}

export interface VideoDetails extends VideoCard {
  manifest_path: string | null;
  manifest_url: string;
  available_qualities: string[];
  likes: number;
  dislikes: number;
  my_reaction: -1 | 0 | 1;
  comment_count: number;
  subscribers: number;
  is_subscribed: boolean;
}

export interface Comment {
  id: string;
  video_id: string;
  content: string;
  created_at: string;
  updated_at: string;
  author: ChannelSummary;
}

export interface UpdateVideoInput {
  title?: string;
  description?: string;
  visibility?: VideoVisibility;
  thumbnail?: string;
}

export interface UpdateProfileInput {
  channel_name?: string;
  description?: string;
  avatar?: string;
}
