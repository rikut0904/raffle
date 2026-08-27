import { getUser } from './cache.js';

export function renderHeader() {
    if (document.querySelector(".admin-navbar")) return;
    
    const nav = document.createElement("nav");
    nav.className = "admin-navbar";
    nav.innerHTML = `
        <div class="wrap">
            <div class="nav-group">
                <a href="/dashboard" class="nav-link" data-path="/dashboard">Dashboard</a>
                <a href="/raffle" class="nav-link" data-path="/raffle">くじ引き</a>
            </div>
            <div class="nav-group">
                <span id="user-email" class="nav-user-email"></span>
                <a id="logout-button" href="/auth/logout" class="btn nav-logout-btn">ログアウト</a>
            </div>
        </div>
    `;
    document.querySelector(".admin-layout").prepend(nav);

    nav.querySelectorAll("[data-path]").forEach((link) => {
        if (link.dataset.path === window.location.pathname) {
            link.classList.add("active");
        }
    });
    
    const user = getUser();
    if (user) document.getElementById("user-email").textContent = user.email;
}
