import { getRaffles, setRaffles } from "./cache.js";
import { renderHeader } from "./components.js";
import { logoutUser } from "./auth.js";

const state = {
    config: null,
    spinning: false,
    excludedIndices: new Set(),
};

const statusElement = document.getElementById("admin-status");
const viewSelection = document.getElementById("view-selection");
const viewPlay = document.getElementById("view-play");
const rafflesList = document.getElementById("raffles-list");
const rafflesEmpty = document.getElementById("raffles-empty");
const box = document.getElementById("raffle-box");
const ticket = document.getElementById("raffle-ticket");
const ticketLabel = document.getElementById("ticket-label");
const spinButton = document.getElementById("spin-button");
const itemList = document.getElementById("item-list");
const resetPoolButton = document.getElementById("reset-pool-button");

function unauthorizedError() {
    const error = new Error("Unauthorized");
    error.code = "UNAUTHORIZED";
    return error;
}

function getItemWeight(item) {
    const parsed = Number.parseInt(item.weight, 10);
    return Number.isFinite(parsed) && parsed > 0 ? parsed : 1;
}

function renderItemList() {
    itemList.innerHTML = "";
    const fragment = document.createDocumentFragment();
    state.config.items.forEach((item, index) => {
        const isExcluded = state.excludedIndices.has(index);
        const div = document.createElement("div");
        div.className = "legend-item";
        if (isExcluded) {
            div.style.opacity = "0.3";
            div.style.textDecoration = "line-through";
            div.style.filter = "grayscale(1)";
        }
        div.innerHTML = `
            <span class="legend-color" style="background: ${item.color}"></span>
            <div style="display: flex; justify-content: space-between; width: 100%;">
                <span>${item.label}</span>
            </div>
        `;
        fragment.appendChild(div);
    });
    itemList.appendChild(fragment);

    // Show reset button only if we have excluded items
    if (resetPoolButton) {
        resetPoolButton.style.display = state.excludedIndices.size > 0 ? "flex" : "none";
    }
}

// --- Actions ---

export function spin() {
    if (state.spinning || !state.config || state.config.items.length === 0) return;
    
    // Filter available items
    const availableIndices = state.config.items
        .map((_, index) => index)
        .filter(index => !state.excludedIndices.has(index));

    if (availableIndices.length === 0) {
        alert("すべての項目が抽出されました。リセットしてください。");
        return;
    }

    state.spinning = true;
    if (spinButton) {
        spinButton.disabled = true;
        spinButton.textContent = "抽選中...";
    }

    // Animation
    ticket.classList.remove("showing");
    box.classList.add("shaking");

    const selectedIndex = pickWeightedIndex(availableIndices);
    const selected = state.config.items[selectedIndex];

    window.setTimeout(() => {
        box.classList.remove("shaking");
        
        // Show ticket
        ticketLabel.textContent = selected.label;
        ticket.style.borderLeft = `8px solid ${selected.color}`;
        ticket.classList.add("showing");

        window.setTimeout(() => {
            state.spinning = false;
            if (spinButton) {
                spinButton.disabled = false;
                spinButton.textContent = "抽選する";
            }
            
            // Add to excluded if prevention is ON
            if (state.config.preventDuplicates) {
                state.excludedIndices.add(selectedIndex);
                renderItemList();
            }
            
            openResultModal(selected);
        }, 800);
    }, 1500);
}

function pickWeightedIndex(availableIndices) {
    const items = availableIndices.map(i => state.config.items[i]);
    const totalWeight = items.reduce((sum, item) => sum + getItemWeight(item), 0);
    
    if (totalWeight <= 0) {
        return availableIndices[Math.floor(Math.random() * availableIndices.length)];
    }

    let threshold = Math.random() * totalWeight;
    for (let i = 0; i < availableIndices.length; i++) {
        threshold -= getItemWeight(items[i]);
        if (threshold < 0) {
            return availableIndices[i];
        }
    }

    return availableIndices[availableIndices.length - 1];
}

window.resetPool = () => {
    if (state.excludedIndices.size === 0) return;
    if (confirm("除外された項目をすべて元に戻しますか？")) {
        state.excludedIndices.clear();
        renderItemList();
    }
};

function openResultModal(selected) {
    let modal = document.getElementById("result-modal");
    if (!modal) {
        modal = document.createElement("div");
        modal.id = "result-modal";
        modal.className = "result-modal";
        modal.innerHTML = `
            <div class="result-modal-backdrop" data-close-modal></div>
            <div class="result-modal-dialog result-modal-dialog-compact" role="dialog" aria-modal="true" aria-labelledby="result-modal-title">
                <div class="result-modal-card">
                    <div>
                        <p class="result-label">選ばれたのは...</p>
                        <h2 id="result-modal-title" class="result-modal-title"></h2>
                    </div>
                    <div class="result-modal-chip">
                        <span id="result-modal-swatch" class="result-modal-chip-swatch"></span>
                        <span id="result-modal-chip-label" class="result-modal-chip-label"></span>
                    </div>
                    <div class="result-modal-actions">
                        <button type="button" class="btn primary" data-close-modal>閉じる</button>
                    </div>
                </div>
            </div>
        `;
        modal.addEventListener("click", (e) => {
            if (e.target.closest("[data-close-modal]")) closeModal();
        });
        document.body.appendChild(modal);
    }

    modal.querySelector("#result-modal-title").textContent = selected.label;
    modal.querySelector("#result-modal-chip-label").textContent = selected.label;
    modal.querySelector("#result-modal-swatch").style.background = selected.color || "var(--accent)";
    modal.classList.add("is-open");
    document.body.classList.add("modal-open");
}

function closeModal() {
    const modal = document.getElementById("result-modal");
    if (modal) {
        modal.classList.remove("is-open");
        document.body.classList.remove("modal-open");
    }
}

function setStatus(message, type = "info") {
    statusElement.textContent = message;
    statusElement.dataset.tone = type;
    statusElement.style.display = message ? "block" : "none";
}

window.toggleFullscreen = async (enable) => {
    const exitBtn = document.querySelector(".exit-presentation");
    const enterBtn = document.getElementById("fullscreen-button");
    if (enable) {
        document.body.classList.add("presentation-mode");
        if (exitBtn) exitBtn.style.display = "block";
        if (enterBtn) enterBtn.style.display = "none";
        
        if (document.documentElement.requestFullscreen) {
            try {
                await document.documentElement.requestFullscreen();
                // Try to lock the Escape key to prevent browser's default exit
                if (navigator.keyboard && navigator.keyboard.lock) {
                    await navigator.keyboard.lock(["Escape"]);
                }
            } catch (err) {
                console.warn("Fullscreen or Keyboard Lock failed:", err);
            }
        }
    } else {
        document.body.classList.remove("presentation-mode");
        if (exitBtn) exitBtn.style.display = "none";
        if (enterBtn) enterBtn.style.display = "block";
        
        // Unlock keyboard and exit fullscreen
        if (navigator.keyboard && navigator.keyboard.unlock) {
            navigator.keyboard.unlock();
        }
        if (document.fullscreenElement && document.exitFullscreen) {
            await document.exitFullscreen().catch(() => {});
        }
    }
};

// --- Data ---

function renderSelectionList(raffles) {
    rafflesList.innerHTML = "";
    if (raffles && raffles.length > 0) {
        rafflesEmpty.style.display = "none";
        const fragment = document.createDocumentFragment();

        raffles.forEach(r => {
            const card = document.createElement("div");
            card.className = "raffle-card";
            card.style.cursor = "pointer";
            card.onclick = () => { window.location.href = `/raffle?id=${r.id}`; };

            const header = document.createElement("div");
            header.className = "raffle-card-header";
            
            const title = document.createElement("strong");
            title.className = "raffle-card-title";
            title.textContent = r.title;

            const desc = document.createElement("p");
            desc.className = "raffle-card-desc";
            desc.textContent = r.description || "説明はありません。";

            header.appendChild(title);
            header.appendChild(desc);

            const stats = document.createElement("div");
            stats.className = "raffle-card-stats";
            
            const count = document.createElement("span");
            count.innerHTML = `<span class="material-symbols-outlined" style="font-size: 1.1rem;">list_alt</span> ${r.items ? r.items.length : 0} 項目`;

            const date = document.createElement("span");
            const formattedDate = new Date(r.updatedAt).toLocaleDateString();
            date.innerHTML = `<span class="material-symbols-outlined" style="font-size: 1.1rem;">calendar_today</span> 更新: ${formattedDate}`;

            stats.appendChild(count);
            stats.appendChild(date);

            const actions = document.createElement("div");
            actions.className = "raffle-card-actions";
            actions.innerHTML = `<button class="btn primary" style="width: 100%"><span class="material-symbols-outlined" style="margin-right: 8px;">play_arrow</span> 実行する</button>`;

            card.appendChild(header);
            card.appendChild(stats);
            card.appendChild(actions);
            fragment.appendChild(card);
        });
        rafflesList.appendChild(fragment);
    } else {
        rafflesEmpty.style.display = "block";
    }
}

async function init() {
    renderHeader();

    const params = new URLSearchParams(window.location.search);
    const id = params.get("id");

    // Sync presentation mode with browser fullscreen state
    document.addEventListener("fullscreenchange", () => {
        if (!document.fullscreenElement && !document.body.classList.contains("modal-open")) {
            if (document.body.classList.contains("presentation-mode")) {
                window.toggleFullscreen(false);
            }
        }
        
        if (!document.fullscreenElement && navigator.keyboard && navigator.keyboard.unlock) {
            navigator.keyboard.unlock();
        }
    });

    document.addEventListener("keydown", (e) => {
        if (e.key === "Escape") {
            if (document.body.classList.contains("modal-open")) {
                closeModal();
                
                e.preventDefault();
                e.stopImmediatePropagation();
                return;
            }

            if (document.body.classList.contains("presentation-mode")) {
                window.toggleFullscreen(false);
                e.preventDefault();
                e.stopImmediatePropagation();
            }
        }
    }, { capture: true });

    try {
        let targetData = null;
        if (id) {
            setStatus("設定を読み込んでいます...");
            const res = await fetch(`/api/dashboard/raffles/${id}`, { credentials: 'include' });
            if (res.status === 401) throw unauthorizedError();
            if (!res.ok) throw new Error("設定が見つかりませんでした");
            targetData = await res.json();
        } else {
            targetData = getRaffles();
            if (!targetData) {
                const res = await fetch("/api/dashboard/raffles", { credentials: 'include', cache: "no-store" });
                if (res.status === 401) throw unauthorizedError();
                targetData = await res.json();
                setRaffles(targetData);
            }
        }

        if (id) {
            state.config = targetData;
            viewPlay.style.display = "block";
            document.getElementById("raffle-title").textContent = state.config.title;
            document.getElementById("raffle-description").textContent = state.config.description || "説明はありません。";
            renderItemList();
            setStatus("");
            } else {
            renderSelectionList(targetData);
            viewSelection.style.display = "block";
            setStatus("");
            }

    } catch (err) {
        if (err.code === "UNAUTHORIZED") {
            window.location.href = "/login";
            return;
        }
        setStatus(err.message, "error");
    }
}

init();
