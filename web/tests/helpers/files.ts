import type { DOMWrapper } from "@vue/test-utils";

/** Puts `files` on a file input and fires `change`, like a user picking them. */
export const pickFiles = async (input: DOMWrapper<Element>, files: File[]) => {
  Object.defineProperty(input.element, "files", { value: files, configurable: true });
  await input.trigger("change");
};

export const makeFile = (name: string, type: string, size = 1024) => {
  const f = new File(["x"], name, { type });
  Object.defineProperty(f, "size", { value: size });
  return f;
};
