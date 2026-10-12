import { http } from "./http";
import type { Comment, Page, UpdateVideoInput, VideoCard, VideoDetails } from "@/types";
import { Constants } from "@/utils/constants";

const paging = (page: number, size = Constants.defaultLimit) => ({ page, size });

class CatalogService {
  browse(search: string, page: number): Promise<Page<VideoCard>> {
    return http.get("/api/videos/browse", { params: { search, ...paging(page) } });
  }
  mine(page: number): Promise<Page<VideoCard>> {
    return http.get("/api/videos/mine", { params: paging(page) });
  }
  liked(page: number): Promise<Page<VideoCard>> {
    return http.get("/api/videos/liked", { params: paging(page) });
  }
  subscriptionFeed(page: number): Promise<Page<VideoCard>> {
    return http.get("/api/videos/subscriptions", { params: paging(page) });
  }
  details(id: string): Promise<VideoDetails> {
    return http.get(`/api/videos/${id}/details`);
  }
  related(id: string, page: number): Promise<Page<VideoCard>> {
    return http.get(`/api/videos/${id}/related`, { params: paging(page, 12) });
  }
  update(id: string, input: UpdateVideoInput): Promise<VideoDetails> {
    return http.put(`/api/videos/${id}`, input);
  }
  remove(id: string): Promise<null> {
    return http.delete(`/api/videos/${id}`);
  }
  recordView(id: string): Promise<null> {
    return http.post(`/api/videos/${id}/view`);
  }
  react(id: string, value: -1 | 0 | 1): Promise<VideoDetails> {
    return http.put(`/api/videos/${id}/reaction`, { value });
  }
  comments(id: string, page: number): Promise<Page<Comment>> {
    return http.get(`/api/videos/${id}/comments`, { params: paging(page) });
  }
  addComment(id: string, content: string): Promise<Comment> {
    return http.post(`/api/videos/${id}/comments`, { content });
  }
  deleteComment(id: string, commentId: string): Promise<null> {
    return http.delete(`/api/videos/${id}/comments/${commentId}`);
  }
}
export default new CatalogService();
