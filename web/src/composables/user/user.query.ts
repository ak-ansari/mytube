import { useQuery } from "@tanstack/vue-query";
import { computed, type MaybeRefOrGetter, toValue } from "vue";
import userService from "@/api/user.service";
import { isAuthenticated } from "@/auth/keycloak";
import { usePagedQuery } from "../usePagedQuery";
import { userKeys } from "./user.keys";

export const useMe = () =>
  useQuery({
    queryKey: userKeys.me(),
    queryFn: () => userService.me(),
    enabled: isAuthenticated,
    staleTime: 5 * 60 * 1000,
  });

export const useMySubscriptions = () =>
  useQuery({
    queryKey: userKeys.subscriptions(),
    queryFn: () => userService.mySubscriptions(),
    enabled: isAuthenticated,
  });

export const useChannel = (id: MaybeRefOrGetter<string>) =>
  useQuery({
    queryKey: computed(() => userKeys.channel(toValue(id))),
    queryFn: () => userService.channel(toValue(id)),
    enabled: computed(() => !!toValue(id)),
    retry: false,
  });

export const useChannelVideos = (id: MaybeRefOrGetter<string>) =>
  usePagedQuery(
    () => userKeys.channelVideos(toValue(id)),
    (page) => userService.channelVideos(toValue(id), page),
    () => !!toValue(id)
  );
