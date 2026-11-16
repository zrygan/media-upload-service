// const video = document.querySelector("video");

function startPreview(video) {
  video.currentTime = 1;
  video.playbackRate = 1;
  video.play();
}

function stopPreview(video) {
  video.currentTime = 0;
  video.playbackRate = 1;
  video.pause();
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

    if (document.fullscreenElement) return;

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

  videoContainer.addEventListener("click", (event) => {
      const video = event.target.closest("video");
      if (!video) return;

      if (event.target.tagName !== 'VIDEO' && document.fullscreenElement) return;

      if(document.fullscreenElement){
        document.exitFullscreen();
      }
      else{
        video.requestFullscreen();
        video.setAttribute("controls", "");
        video.volume = 0.5;
      }

  });

  document.addEventListener("fullscreenchange", () =>{
    const videos = document.querySelectorAll('video');
    if(!document.fullscreenElement){
      videos.forEach(v => {
        v.removeAttribute('controls');
        v.dataset.previewing = "0";
      });
    }
  });

  try {
    const response = await fetch('/api/videos/');
    const videoURLs = await response.json();

    videoContainer.innerHTML = '';

    videoURLs.forEach(v => {
      const videoElement = `
      <div class = "video-item" controls>
          <video width="320" height="240" >
            <source src="${v.url}" type="video/mp4">
            yo video wont load
          </video>
          <p>${v.title}</p>
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