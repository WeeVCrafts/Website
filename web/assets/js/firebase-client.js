/* Firebase customer identity is exchanged for a first-party HttpOnly session.
 * Browser auth uses memory persistence; tokens are never logged or stored here. */
(function (global) {
  "use strict";
  const SDK = "https://www.gstatic.com/firebasejs/13.0.0/";
  let readyPromise;
  let pendingLink;
  async function ready() {
    if (!readyPromise) readyPromise = (async () => {
      const response = await fetch("/api/v1/firebase/config", {credentials: "same-origin"});
      if (!response.ok) return null;
      const settings = await response.json();
      if (!settings.enabled) return null;
      const [appSDK, authSDK] = await Promise.all([import(SDK + "firebase-app.js"), import(SDK + "firebase-auth.js")]);
      const app = appSDK.initializeApp(settings.config);
      const auth = authSDK.getAuth(app);
      await authSDK.setPersistence(auth, authSDK.inMemoryPersistence);
      return {settings, app, auth, authSDK};
    })().catch(() => {readyPromise = undefined; throw new Error("Secure Google sign-in could not load. Check your connection and try again.");});
    return readyPromise;
  }
  function safeError(error) {
    const code = error && error.code;
    if (code === "auth/popup-closed-by-user" || code === "auth/cancelled-popup-request") return new Error("Google sign-in was cancelled. You can try again.");
    if (code === "auth/popup-blocked") return new Error("Allow the Google sign-in popup, then try again.");
    if (code === "auth/email-already-in-use") return new Error("An account already uses this email. Sign in or reset your password.");
    if (code === "auth/too-many-requests") return new Error("Too many attempts. Please wait before trying again.");
    if (code === "auth/weak-password") return new Error("Choose a password with at least 12 characters.");
    if (code === "auth/invalid-action-code" || code === "auth/expired-action-code") return new Error("This email link is invalid or expired. Request a new one.");
    if (code === "auth/network-request-failed") return new Error("The authentication service could not be reached. Please try again.");
    if (code && code.startsWith("auth/")) return new Error("Sign-in could not be completed. Check your email and password, or reset your password.");
    return error;
  }
  async function exchange(api, user, input) {
    const body = {id_token: await user.getIdToken(true), next: input.next || "", accepted_terms: input.acceptedTerms === true, phone: input.phone || "", link_password: input.linkPassword || ""};
    try {
      const result = await api.call("/api/v1/auth/firebase/session", {method:"POST", authenticated:true, body});
      api.csrf = result.csrf_token;
      pendingLink = undefined;
      return result;
    } catch (error) {
      if (error.code === "LINK_REQUIRED") {
        if (!user.emailVerified) {
          throw new Error("Verify your email using the link in your inbox, then sign in again to connect your existing account securely.");
        }
        pendingLink = {user, input};
        const panel = document.querySelector("[data-firebase-link]");
        if (panel) {panel.hidden = false; panel.querySelector("input").focus();}
      }
      throw error;
    }
  }
  async function signIn(api, input, legacy) {
    // Preserve existing role credentials and existing customer passwords.
    try {return await legacy();} catch (error) {
      if (error.code !== "INVALID_CREDENTIALS") throw error;
    }
    const f = await ready();
    if (!f) throw new Error("Email or password is incorrect.");
    try {return await exchange(api, (await f.authSDK.signInWithEmailAndPassword(f.auth,input.email,input.password)).user,input);} catch(error) {throw safeError(error);}
  }
  async function signUp(api,input,legacy) {
    const f = await ready(); if (!f) return legacy();
    if (!input.acceptedTerms) throw new Error("Accept the privacy notice and terms to create your account.");
    if (input.password.length < 12) throw new Error("Choose a password with at least 12 characters.");
    try {
      let user;
      try {
        user = (await f.authSDK.createUserWithEmailAndPassword(f.auth,input.email,input.password)).user;
        await f.authSDK.updateProfile(user,{displayName:input.displayName});
      } catch (error) {
        if (error.code !== "auth/email-already-in-use") throw error;
        // Retry a registration interrupted before the server saved its customer
        // record. Password proof is still required; email matching never links.
        user = (await f.authSDK.signInWithEmailAndPassword(f.auth,input.email,input.password)).user;
      }
      let verificationFailed = false;
      if (!user.emailVerified) {
        try {await api.call("/api/v1/auth/firebase/verification",{method:"POST",authenticated:true,body:{id_token:await user.getIdToken(true)}});}
        catch (_) {verificationFailed = true;}
      }
      try {
        const result = await exchange(api,user,input);
        if (verificationFailed) {
          try {sessionStorage.setItem("weevcrafts_auth_notice","Your account was created, but the verification email could not be sent. Retry from Account → Sessions.");} catch (_) {}
        }
        return result;
      } catch(error) {
        if (verificationFailed && !user.emailVerified) throw new Error("The verification email could not be sent. Please retry registration later to connect this existing account.");
        throw error;
      }
    } catch(error) {throw safeError(error);}
  }
  async function recover(email,legacy) {
    // The server selects native/Firebase ownership and generates private action
    // links. Email HTML and delivery are now owned by the application sender.
    return legacy();
  }
  async function action(password,legacy) {
    const code = new URLSearchParams(global.location.search).get("oobCode");
    if (!code) return legacy();
    const f = await ready(); if (!f) throw new Error("Email actions are unavailable.");
    try {
      if (password) {await f.authSDK.verifyPasswordResetCode(f.auth,code); await f.authSDK.confirmPasswordReset(f.auth,code,password);}
      else await f.authSDK.applyActionCode(f.auth,code);
    } catch(error) {throw safeError(error);}
  }
  function message(container,text,error) {
    let status = container.querySelector("[data-firebase-status]");
    if (!status) {status=document.createElement("p");status.dataset.firebaseStatus="";status.setAttribute("role","status");status.setAttribute("aria-live","polite");container.append(status);}
    status.textContent=text; status.className=error?"form-message error":"form-message";
  }
  function setGoogleAvailability(panel,enabled,text) {
    const button=panel.querySelector("[data-google-signin]");
    const terms=panel.querySelector(".google-terms-label");
    const note=panel.querySelector("[data-firebase-config-note]");
    if (button) {button.disabled=!enabled;button.setAttribute("aria-disabled",String(!enabled));}
    if (terms) terms.hidden=!enabled;
    if (note) {note.hidden=enabled;note.textContent=text||"";}
  }
  async function google(button) {
    const panel=button.closest("[data-firebase-google-panel]");
    const terms=panel.querySelector("[data-google-terms]").checked;
    // Show terms before contacting Google rather than auto-granting consent.
    if (!terms) {message(panel,"Accept the privacy notice and terms before continuing.",true);return;}
    button.disabled=true;
    try {
      const f=await ready();if(!f)throw new Error("Google sign-in is unavailable.");
      const provider=new f.authSDK.GoogleAuthProvider();provider.setCustomParameters({prompt:"select_account"});
      const user=(await f.authSDK.signInWithPopup(f.auth,provider)).user;
      const api=global.weevcraftsBackend || await global.WeevcraftsBackend.create();
      const result=await exchange(api,user,{acceptedTerms:terms,next:new URLSearchParams(global.location.search).get("next")||""});
      global.location.assign(result.redirect_to||"/products");
    } catch(error) {message(panel,safeError(error).message,true);} finally {button.disabled=false;}
  }
  async function start() {
    const panels=document.querySelectorAll("[data-firebase-google-panel], [data-firebase-push-panel], [data-firebase-diagnostics]");
    if (!panels.length) return;
    try {
      const googlePanels=document.querySelectorAll("[data-firebase-google-panel]");
      const f=await ready();
      if(!f) {
        googlePanels.forEach(panel=>setGoogleAvailability(panel,false,"Google sign-in is not configured in this local workspace. Continue with email to use the local accounts."));
        return;
      }
      googlePanels.forEach(panel=>{setGoogleAvailability(panel,true,"");panel.querySelector("[data-google-signin]").addEventListener("click",event=>google(event.currentTarget));});
      const link=document.querySelector("[data-firebase-link]");
      if (link)link.addEventListener("submit",async event=>{
        event.preventDefault(); if(!pendingLink)return;
        const button=link.querySelector("button");button.disabled=true;
        try{const api=global.weevcraftsBackend || await global.WeevcraftsBackend.create();const result=await exchange(api,pendingLink.user,{...pendingLink.input,linkPassword:link.querySelector("input").value});global.location.assign(result.redirect_to||"/products");}
        catch(error){message(link,safeError(error).message,true);}finally{link.querySelector("input").value="";button.disabled=false;}
      });
      await setupPush(f);
      document.querySelectorAll("[data-firebase-diagnostics]").forEach(panel=>{
        panel.hidden=false;
        panel.querySelector("button").addEventListener("click",async()=>{
          try{const sdk=await import(SDK+"firebase-performance.js");const perf=sdk.getPerformance(f.app);perf.dataCollectionEnabled=true;perf.instrumentationEnabled=true;message(panel,"Performance diagnostics enabled for this visit. They stop when you leave or reload the page.",false);}
          catch(_){message(panel,"Diagnostics could not be enabled.",true);}
        });
      });
    }catch(_){
      document.querySelectorAll("[data-firebase-google-panel]").forEach(panel=>setGoogleAvailability(panel,false,"Google sign-in could not be loaded. Continue with email or try again later."));
      /* Existing first-party forms remain available during a CDN outage. */
    }
  }
  async function setupPush(f) {
    const panel=document.querySelector("[data-firebase-push-panel]");if(!panel||!f.settings.vapidKey)return;
    panel.hidden=false;
    const enable=panel.querySelector("[data-push-enable]"),disable=panel.querySelector("[data-push-disable]"),test=panel.querySelector("[data-push-test]");
    const api=global.weevcraftsBackend || await global.WeevcraftsBackend.create();
    const update=async()=>{const state=await api.call("/api/v1/customer/push");enable.hidden=state.enabled;disable.hidden=!state.enabled;test.hidden=!state.enabled;};
    await update();
    enable.addEventListener("click",async()=>{
      enable.disabled=true;
      try{
        if (!global.isSecureContext||!("Notification"in global)||!("serviceWorker"in navigator))throw new Error("This browser needs HTTPS and notification support.");
        if(await Notification.requestPermission()!=="granted")throw new Error("Notifications are blocked. You can enable them in your browser settings.");
        const sdk=await import(SDK+"firebase-messaging.js");if(!await sdk.isSupported())throw new Error("Push notifications are not supported in this browser.");
        const registration=await navigator.serviceWorker.register("/firebase-messaging-sw.js");
        const token=await sdk.getToken(sdk.getMessaging(f.app),{vapidKey:f.settings.vapidKey,serviceWorkerRegistration:registration});
        await api.call("/api/v1/customer/push",{method:"POST",authenticated:true,body:{token}});
        sdk.onMessage(sdk.getMessaging(f.app),()=>message(panel,"A new account update is available in your notifications.",false));
        await update();message(panel,"Private account updates enabled on this signed-in browser.",false);
      }catch(error){message(panel,safeError(error).message,true);}finally{enable.disabled=false;}
    });
    disable.addEventListener("click",async()=>{
      disable.disabled=true;
      try{await api.call("/api/v1/customer/push",{method:"DELETE",authenticated:true});await update();message(panel,"Notifications disabled for this signed-in browser.",false);}
      catch(_){message(panel,"Notifications could not be disabled. Please try again.",true);}finally{disable.disabled=false;}
    });
    test.addEventListener("click",async()=>{
      test.disabled=true;
      try{await api.call("/api/v1/customer/push/test",{method:"POST",authenticated:true,body:{}});message(panel,"Test update sent. Background the app to check your browser notification.",false);}
      catch(_){message(panel,"Test delivery failed. Please try again.",true);}finally{test.disabled=false;}
    });
  }
  global.WeevcraftsFirebase={signIn,signUp,recover,action};
  // The Firebase service worker forwards foreground messages to visible tabs.
  // Handle its public message event without loading auth or analytics on every
  // shopping page. The payload body is deliberately never rendered.
  if ("serviceWorker" in navigator) navigator.serviceWorker.addEventListener("message",event=>{
    if (!event.data || !event.data.isFirebaseMessaging || event.data.messageType !== "push-received") return;
    const bell=document.querySelector('a[href="/account/notifications"]');
    if (bell) {bell.setAttribute("aria-label","Notifications: a new account update is available");bell.animate([{opacity:0.6},{opacity:1}],{duration:matchMedia("(prefers-reduced-motion: reduce)").matches?0:180});}
  });
  document.addEventListener("DOMContentLoaded",start);
})(window);
