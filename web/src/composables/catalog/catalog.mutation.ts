import { useMutation, useQueryClient } from "@tanstack/vue-query";
import catalogService from "@/api/catalog.service";
import type { UpdateVideoInput, VideoDetails } from "@/types";
import { catalogKeys } from "./catalog.keys";

export const useUpdateVideo = () => {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, input }: { id: string; input: UpdateVideoInput }) => catalogService.update(id, input),
    onSuccess: () => qc.invalidateQueries({ queryKey: catalogKeys.all }),
  });
};

export const useDeleteVideo = () => {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => catalogService.remove(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: catalogKeys.all }),
  });
};

export const useReactToVideo = () => {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, value }: { id: string; value: -1 | 0 | 1 }) => catalogService.react(id, value),
    onSuccess: (details: VideoDetails) => {
      qc.setQueryData(catalogKeys.details(details.id), details);
      qc.invalidateQueries({ queryKey: catalogKeys.liked() });
    },
  });
};

export const useAddComment = (videoId: () => string) => {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (content: string) => catalogService.addComment(videoId(), content),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: catalogKeys.comments(videoId()) });
      qc.invalidateQueries({ queryKey: catalogKeys.details(videoId()) });
    },
  });
};

export const useDeleteComment = (videoId: () => string) => {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (commentId: string) => catalogService.deleteComment(videoId(), commentId),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: catalogKeys.comments(videoId()) });
      qc.invalidateQueries({ queryKey: catalogKeys.details(videoId()) });
    },
  });
};
