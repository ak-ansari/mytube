import { http } from "./http";
import type { Channel, Page, Profile, UpdateProfileInput, VideoCard } from "@/types";
import { Constants } from "@/utils/constants";

class UserService {
  me(): Promise<Profile> {
    return http.get("/api/users/me");
  }
  updateMe(input: UpdateProfileInput): Promise<Profile> {
    return http.put("/api/users/me", input);
  }
  mySubscriptions(): Promise<Channel[]> {
    return http.get("/api/users/me/subscriptions");
  }
  channel(id: string): Promise<Channel> {
    return http.get(`/api/channels/${id}`);
  }
  channelVideos(id: string, page: number): Promise<Page<VideoCard>> {
    return http.get(`/api/channels/${id}/videos`, { params: { page, size: Constants.defaultLimit } });
  }
  subscribe(id: string): Promise<Channel> {
    return http.post(`/api/channels/${id}/subscribe`);
  }
  unsubscribe(id: string): Promise<Channel> {
    return http.delete(`/api/channels/${id}/subscribe`);
  }
}
export default new UserService();
