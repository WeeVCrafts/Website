/* Account updates contain no personal or order details. Never cache account pages. */
importScripts("https://www.gstatic.com/firebasejs/13.0.0/firebase-app-compat.js");
importScripts("https://www.gstatic.com/firebasejs/13.0.0/firebase-messaging-compat.js");
if (WEEVCRAFTS_FIREBASE_CONFIG.apiKey) {
  firebase.initializeApp(WEEVCRAFTS_FIREBASE_CONFIG);
  firebase.messaging().onBackgroundMessage(payload=>self.registration.showNotification("WeevCrafts",{
    body:"You have a new account update. Open WeevCrafts to view it securely.",icon:"/assets/images/favicon.svg",tag:"weevcrafts-account-update",data:{url:"/account/notifications"}
  }));
}
self.addEventListener("notificationclick",event=>{
  event.notification.close();event.waitUntil(clients.openWindow("/account/notifications"));
});
