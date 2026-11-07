/* CFData 最小 Service Worker（A8，学习 subs-check-pro sw.js）：
   仅满足 PWA「可安装到主屏」判据——零缓存、网络透传、不提供离线能力，
   断网即不可用；静态资源仍由服务端 no-cache 策略控制（见 9d01c3d）。 */
self.addEventListener('install', () => {
    self.skipWaiting();
});

self.addEventListener('activate', (event) => {
    event.waitUntil(self.clients.claim());
});

self.addEventListener('fetch', (event) => {
    event.respondWith(fetch(event.request));
});
