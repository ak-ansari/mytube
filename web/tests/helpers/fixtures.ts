import { VideoStatus, VideoVisibility, type Channel, type Comment, type Page, type Profile, type VideoCard, type VideoDetails } from "@/types";

export const channelSummary = (over: Partial<VideoCard["channel"]> = {}): VideoCard["channel"] => ({
  id: "chan-1",
  user_name: "alice",
  channel_name: "Alice Films",
  avatar: null,
  avatar_url: "",
  ...over,
});

export const videoCard = (over: Partial<VideoCard> = {}): VideoCard => ({
  id: "vid-1",
  title: "My first video",
  description: "A description",
  thumbnail: "thumbnails/vid-1/vid-1.jpeg",
  thumbnail_url: "http://minio.test/thumb.jpg",
  visibility: VideoVisibility.Public,
  status: VideoStatus.Published,
  views: 1234,
  duration_seconds: 75,
  created_at: new Date(Date.now() - 2 * 24 * 3600 * 1000).toISOString(),
  updated_at: new Date(Date.now() - 2 * 24 * 3600 * 1000).toISOString(),
  channel: channelSummary(),
  ...over,
});

export const videoDetails = (over: Partial<VideoDetails> = {}): VideoDetails => ({
  ...videoCard(),
  manifest_path: "segments/vid-1/master.m3u8",
  manifest_url: "http://hls.test/videos/tok/1/vid-1/master.m3u8",
  available_qualities: ["360p"],
  likes: 1500,
  dislikes: 2,
  my_reaction: 0,
  comment_count: 2,
  subscribers: 42,
  is_subscribed: false,
  ...over,
});

export const channel = (over: Partial<Channel> = {}): Channel => ({
  ...channelSummary(),
  description: "Channel about films",
  subscribers: 1200,
  video_count: 3,
  is_subscribed: false,
  created_at: "2024-01-15T10:00:00Z",
  ...over,
});

export const profile = (over: Partial<Profile> = {}): Profile => ({ ...channel(), email: "alice@example.com", ...over });

export const comment = (over: Partial<Comment> = {}): Comment => ({
  id: "c-1",
  video_id: "vid-1",
  content: "Nice video!",
  created_at: new Date(Date.now() - 3600 * 1000).toISOString(),
  updated_at: new Date(Date.now() - 3600 * 1000).toISOString(),
  author: channelSummary({ id: "user-2", user_name: "bob", channel_name: "Bob" }),
  ...over,
});

export const page = <T>(items: T[], over: Partial<Page<T>> = {}): Page<T> => ({ items, page: 1, size: 20, has_more: false, ...over });
