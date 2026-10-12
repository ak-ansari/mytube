import fileService from "@/api/file.service";
import { getExtension } from "@/utils";
import { v4 } from "uuid";

/** Image formats the API accepts for thumbnails and avatars (it checks the key's extension). */
export const IMAGE_ACCEPT = "image/jpeg,image/png,image/webp,image/gif";

const IMAGE_EXTENSIONS = ["jpg", "jpeg", "png", "webp", "gif"];

/** Whether the API will accept this file as a thumbnail/avatar (by MIME type, else by extension). */
export const isAcceptedImage = (file: File) =>
  file.type ? IMAGE_ACCEPT.split(",").includes(file.type) : IMAGE_EXTENSIONS.includes(getExtension(file.name).toLowerCase());

/** Extension for the object key: the file name's, else the MIME subtype ("image/png" -> "png"). */
const keyExtension = (file: File) => (getExtension(file.name) || file.type.split("/")[1] || "bin").toLowerCase();

/** Uploads a file straight to object storage under `${prefix}/<uuid>.<ext>` and returns its key. */
export const uploadObject = async (prefix: string, file: File, onProgress?: (percent: number) => void): Promise<string> => {
  const key = `${prefix}/${v4()}.${keyExtension(file)}`;
  const { url } = await fileService.getPresignedUrl(key, file.size);
  await fileService.uploadToS3(url, file, onProgress);
  return key;
};
