import axios from "axios";
import { http } from "./http";
class FileService {
  getPresignedUrl(
    key: string,
    size: number
  ): Promise<{ key: string; url: string }> {
    return http.get("/api/files/presign", {
      params: {
        key,
        size,
      },
    });
  }

  uploadToS3 = (
    presignedUrl: string,
    file: File,
    onProgress?: (percent: number) => void,
    signal?: AbortSignal
  ) => {
    return axios.put(presignedUrl, file, {
      headers: {
        "Content-Type": file.type, // IMPORTANT
      },
      signal,
      onUploadProgress: (event) => {
        if (!event.total) return;
        const percent = Math.round((event.loaded / event.total) * 100);
        onProgress?.(percent);
      },
    });
  };
}
export default new FileService();
