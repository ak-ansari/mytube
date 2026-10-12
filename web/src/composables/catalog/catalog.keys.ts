export const catalogKeys = {
  all: ["catalog"] as const,
  browse: (search: string) => ["catalog", "browse", search],
  mine: () => ["catalog", "mine"],
  liked: () => ["catalog", "liked"],
  subscriptionFeed: () => ["catalog", "subscription_feed"],
  details: (id: string) => ["catalog", "details", id],
  related: (id: string) => ["catalog", "related", id],
  comments: (id: string) => ["catalog", "comments", id],
};
