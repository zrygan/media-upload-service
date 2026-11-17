// Hover: plays the first 10 seconds, muted (browsers block sound until
// the user clicks). Click: plays the whole video fullscreen with sound
// and controls. Esc leaves fullscreen.

const PREVIEW_MS = 10000;
const REFRESH_MS = 3000;

let previewTimer = null;

function startPreview(video) {
  clearTimeout(previewTimer);
  video.muted = true;
  video.currentTime = 0;
  video.play().catch(() => {});
  previewTimer = setTimeout(() => stopPreview(video), PREVIEW_MS);
}

function stopPreview(video) {
  clearTimeout(previewTimer);
  previewTimer = null;
  video.pause();
  video.currentTime = 0;
}

function playFull(video) {
  // without this, the preview timer would stop the video after 10s
  clearTimeout(previewTimer);
  previewTimer = null;
  video.controls = true;
  video.muted = false;
  video.volume = 0.5;
  video.currentTime = 0;
  video.play().catch(() => {});
}

// addVideo builds the item with textContent, not innerHTML, because
// the title is a file name sent by a producer and must not run as HTML
function addVideo(container, v) {
  const item = document.createElement("div");
  item.className = "video-item";

  const video = document.createElement("video");
  video.width = 320;
  video.height = 240;
  video.preload = "metadata";
  video.muted = true;
  video.src = v.url;

  const title = document.createElement("p");
  title.textContent = v.title;

  item.append(video, title);
  container.append(item);
}

document.addEventListener("DOMContentLoaded", () => {
  const container = document.querySelector(".video-grid-container");
  const status = document.querySelector("#status");
  if (!container) {
    console.error("could not find the target container");
    return;
  }

  container.addEventListener("mouseover", (event) => {
    const video = event.target.closest("video");
    if (!video || document.fullscreenElement) return;
    if (video.dataset.previewing === "1") return;
    video.dataset.previewing = "1";
    startPreview(video);
  });

  container.addEventListener("mouseout", (event) => {
    const video = event.target.closest("video");
    if (!video || document.fullscreenElement) return;
    video.dataset.previewing = "0";
    stopPreview(video);
  });

  container.addEventListener("click", (event) => {
    const video = event.target.closest("video");
    // in fullscreen, clicks go to the video's own controls
    if (!video || document.fullscreenElement) return;
    playFull(video);
    video.requestFullscreen().catch(() => {}); // still plays if refused
  });

  document.addEventListener("fullscreenchange", () => {
    if (document.fullscreenElement) return;
    document.querySelectorAll("video").forEach((v) => {
      if (!v.controls) return;
      v.controls = false;
      v.dataset.previewing = "0";
      v.muted = true;
      stopPreview(v);
    });
  });

  // urls already on the page; the list only grows, oldest first
  const shown = new Set();

  async function refresh() {
    try {
      const response = await fetch("/api/videos/");
      if (!response.ok) throw new Error(response.statusText);
      const videos = await response.json();
      for (const v of videos) {
        if (shown.has(v.url)) continue;
        shown.add(v.url);
        addVideo(container, v);
      }
      status.textContent = shown.size === 0 ? "No videos yet." : "";
    } catch (err) {
      console.error("failed to load videos: ", err);
      status.textContent = "Error loading videos.";
    }
  }

  refresh();
  setInterval(refresh, REFRESH_MS);
});
