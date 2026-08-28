import { clearCache } from "./cache.js";

export async function logoutUser() {
    clearCache();
    // The backend clears the local session, then redirects the browser to
    // Common ID so its central session is also invalidated.
    window.location.replace("/auth/logout");
}
