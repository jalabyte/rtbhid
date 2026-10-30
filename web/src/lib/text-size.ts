export type TextSize = "normal" | "large";

const KEY = "rtbh-text-size";

export function getTextSize(): TextSize {
  try {
    return localStorage.getItem(KEY) === "large" ? "large" : "normal";
  } catch {
    return "normal";
  }
}

export function setTextSize(size: TextSize) {
  try {
    if (size === "large") localStorage.setItem(KEY, size);
    else localStorage.removeItem(KEY);
  } catch {
    // not persisted; still applied for this page
  }
  document.documentElement.classList.toggle("text-large", size === "large");
}
