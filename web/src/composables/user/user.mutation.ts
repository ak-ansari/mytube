import { useMutation, useQueryClient } from "@tanstack/vue-query";
import userService from "@/api/user.service";
import type { Channel, UpdateProfileInput } from "@/types";
import { catalogKeys } from "../catalog/catalog.keys";
import { userKeys } from "./user.keys";

export const useUpdateProfile = () => {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: UpdateProfileInput) => userService.updateMe(input),
    onSuccess: (profile) => {
      qc.setQueryData(userKeys.me(), profile);
      qc.invalidateQueries({ queryKey: userKeys.channel(profile.id) });
      qc.invalidateQueries({ queryKey: catalogKeys.all });
    },
  });
};

/** Subscribes when `subscribe` is true, otherwise unsubscribes. */
export const useToggleSubscription = () => {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ channelId, subscribe }: { channelId: string; subscribe: boolean }) =>
      subscribe ? userService.subscribe(channelId) : userService.unsubscribe(channelId),
    onSuccess: (channel: Channel) => {
      qc.setQueryData(userKeys.channel(channel.id), channel);
      qc.invalidateQueries({ queryKey: userKeys.subscriptions() });
      qc.invalidateQueries({ queryKey: catalogKeys.subscriptionFeed() });
      qc.invalidateQueries({ queryKey: ["catalog", "details"] });
    },
  });
};
