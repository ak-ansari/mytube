import { http } from "./http";
import type { UploadVideoDto, Video, VideoConfirmDto,PresignedResult } from "@/types";

class VideoService {
  presignedUrl(video: UploadVideoDto): Promise<PresignedResult> {
    return http.post("/api/videos/upload/presign", video);
  }
  getSuggestions(search: string): Promise<string[]> {
    return http.get("/api/videos/suggest", {
      params: {
        search,
      },
    });
  }
  confirmVideo({id,video}: {id: string, video: VideoConfirmDto}): Promise<Video> {
    return http.post(`/api/videos/${id}/confirm`, video);
  }
}
export default new VideoService();
