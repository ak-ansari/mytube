export * from "./constants";
export * from "./format";

/** "clip.MP4" -> "MP4"; "" when the name has no extension ("avatar", ".env", "file."). */
export const getExtension = (fileName: string): string => {
  const dot = fileName?.lastIndexOf(".") ?? -1;
  return dot > 0 ? fileName.slice(dot + 1) : "";
};
export const stripExtension = (fileName: string) => fileName.replace(/\.[^.]+$/, "");
