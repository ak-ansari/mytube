export const userKeys = {
  me: () => ["user", "me"],
  subscriptions: () => ["user", "subscriptions"],
  channel: (id: string) => ["user", "channel", id],
  channelVideos: (id: string) => ["user", "channel_videos", id],
};
