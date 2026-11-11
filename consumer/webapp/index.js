// const video = document.querySelector("video");

function startPreview(video) {
  video.muted = true;
  video.currentTime = 1;
  video.playbackRate = 1;
  video.play();
}

function stopPreview(video) {
  video.currentTime = 0;
  video.playbackRate = 1;
  video.pause();    
  video.setAttribute("controls", "");
}

let previewTimeout = null;

document.addEventListener('DOMContentLoaded', async () => {
  const videoContainer = document.querySelector('.video-grid-container');
  
  if (!videoContainer) {
    console.error("could not find the target container");
    return;
  }

  videoContainer.addEventListener("mouseover", (event) => {
    const video = event.target.closest("video");
    if (!video) return;

    if(video.dataset.previewing === "1") return;
    video.dataset.previewing = "1";

    startPreview(video);
    previewTimeout = setTimeout(() => stopPreview(video), 10000);
  });

  videoContainer.addEventListener("mouseout", (event) => {
    const video = event.target.closest("video");
    if (!video) return;

    video.dataset.previewing = "0";
    clearTimeout(previewTimeout);
    previewTimeout = null;
    stopPreview(video);
  });

  try {
    const response = await fetch('/api/videos/');
    const videoURLs = await response.json();

    videoContainer.innerHTML = '';

    videoURLs.forEach(url => {
      const videoElement = `
      <div class = "video-item" controls>
          <video width="320" height="240" >
            <source src="${url}" type="video/mp4">
            yo video wont load
          </video>
      </div>
      `;

      videoContainer.innerHTML += videoElement;
    });
  } catch(err) {
    console.error("failed to load videos: ", err);
    videoContainer.innerHTML = `
      <p>error loading videos</p>
    `;
  }
});