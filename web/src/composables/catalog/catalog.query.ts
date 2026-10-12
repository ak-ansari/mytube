import { useQuery } from "@tanstack/vue-query";
import { computed, type MaybeRefOrGetter, toValue } from "vue";
import catalogService from "@/api/catalog.service";
import { isAuthenticated } from "@/auth/keycloak";
import { usePagedQuery } from "../usePagedQuery";
import { catalogKeys } from "./catalog.keys";

export const useBrowseVideos = (search: MaybeRefOrGetter<string> = "") =>
  usePagedQuery(
    () => catalogKeys.browse(toValue(search)),
    (page) => catalogService.browse(toValue(search), page)
  );

export const useMyVideos = () => usePagedQuery(catalogKeys.mine(), (page) => catalogService.mine(page), isAuthenticated);

export const useLikedVideos = () =>
  usePagedQuery(catalogKeys.liked(), (page) => catalogService.liked(page), isAuthenticated);

export const useSubscriptionFeed = () =>
  usePagedQuery(catalogKeys.subscriptionFeed(), (page) => catalogService.subscriptionFeed(page), isAuthenticated);

export const useRelatedVideos = (id: MaybeRefOrGetter<string>) =>
  usePagedQuery(
    () => catalogKeys.related(toValue(id)),
    (page) => catalogService.related(toValue(id), page),
    () => !!toValue(id)
  );

export const useComments = (id: MaybeRefOrGetter<string>) =>
  usePagedQuery(
    () => catalogKeys.comments(toValue(id)),
    (page) => catalogService.comments(toValue(id), page),
    () => !!toValue(id)
  );

export const useVideoDetails = (id: MaybeRefOrGetter<string>) =>
  useQuery({
    queryKey: computed(() => catalogKeys.details(toValue(id))),
    queryFn: () => catalogService.details(toValue(id)),
    enabled: computed(() => !!toValue(id)),
    retry: false,
  });
