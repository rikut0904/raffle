const COLORS = ["#ef476f", "#ff9f1c", "#ffd166", "#06d6a0", "#118ab2", "#7b2cbf"];
const DEFAULT_ITEMS = [
    { label: "参加者A", color: "#ef476f", weight: 1 },
    { label: "参加者B", color: "#ff9f1c", weight: 1 },
    { label: "参加者C", color: "#ffd166", weight: 1 },
    { label: "参加者D", color: "#06d6a0", weight: 1 },
    { label: "参加者E", color: "#118ab2", weight: 1 },
];

const state = {
    items: [],
    history: [],
    spinning: false,
};

// --- Storage Logic ---

function loadJSONFromStorage(key, fallback) {
    const raw = localStorage.getItem(key);
    if (!raw) {
        return fallback;
    }

    try {
        return JSON.parse(raw);
    } catch (_error) {
        localStorage.removeItem(key);
        return fallback;
    }
}

function loadFromCache() {
    state.items = loadJSONFromStorage("raffle-items", loadJSONFromStorage("kujibiki-items", [...DEFAULT_ITEMS]));
    state.history = loadJSONFromStorage("raffle-history", loadJSONFromStorage("kujibiki-history", []));
}

function saveToCache() {
    localStorage.setItem("raffle-items", JSON.stringify(state.items));
    localStorage.setItem("raffle-history", JSON.stringify(state.history));
}

// --- UI Rendering ---

export function renderItems() {
    const container = document.getElementById("item-list");
    if (!container) return;

    container.innerHTML = "";
    const fragment = document.createDocumentFragment();

    state.items.forEach((item, index) => {
        const div = document.createElement("div");
        div.className = "legend-item";

        const colorSpan = document.createElement("span");
        colorSpan.className = "legend-color";
        colorSpan.style.background = item.color;

        const contentDiv = document.createElement("div");
        contentDiv.style = "display: flex; justify-content: space-between; width: 100%; align-items: center;";

        const labelSpan = document.createElement("span");
        labelSpan.textContent = item.label;

        const removeBtn = document.createElement("button");
        removeBtn.type = "button";
        removeBtn.innerHTML = "&times;";
        removeBtn.style = "background: none; border: none; color: #ef476f; cursor: pointer; font-size: 1.2rem; padding: 0 8px;";
        removeBtn.onclick = () => window.removeItem(index);

        contentDiv.appendChild(labelSpan);
        contentDiv.appendChild(removeBtn);
        div.appendChild(colorSpan);
        div.appendChild(contentDiv);
        fragment.appendChild(div);
    });

    container.appendChild(fragment);
}

export function renderHistory() {
    const container = document.getElementById("history-list");
    if (!container) return;

    if (state.history.length === 0) {
        container.innerHTML = `<p style="color: var(--text-muted); padding: 20px; text-align: center;">履歴はありません</p>`;
        return;
    }

    container.innerHTML = "";
    const fragment = document.createDocumentFragment();

    state.history.forEach(item => {
        const article = document.createElement("article");
        article.className = "intro-raffle-card";

        const leftDiv = document.createElement("div");
        leftDiv.style = "display: flex; align-items: center; gap: 12px;";

        const dot = document.createElement("span");
        dot.className = "intro-color-dot";
        dot.style.background = item.color;

        const label = document.createElement("h3");
        label.style = "margin: 0; font-size: 1.1rem; font-weight: 600;";
        label.textContent = item.label;

        leftDiv.appendChild(dot);
        leftDiv.appendChild(label);

        const time = document.createElement("small");
        time.style = "color: var(--text-muted); font-weight: 500;";
        time.textContent = new Date(item.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });

        article.appendChild(leftDiv);
        article.appendChild(time);
        fragment.appendChild(article);
    });

    container.appendChild(fragment);
}

// --- Actions ---

export function addItem() {
    const labelInput = document.getElementById("new-item-label");
    const weightInput = document.getElementById("new-item-weight");
    const label = labelInput.value.trim();
    if (!label) return;
    
    let weight = 1;
    if (weightInput) {
        weight = parseInt(weightInput.value, 10);
        if (isNaN(weight) || weight < 1) weight = 1;
        weightInput.value = "1";
    }

    const color = COLORS[state.items.length % COLORS.length];
    state.items.push({ label, color, weight });
    labelInput.value = "";
    
    saveToCache();
    renderItems();
}

export function removeItem(index) {
    state.items.splice(index, 1);
    saveToCache();
    renderItems();
}

export function resetItems() {
    if (confirm("項目を初期状態に戻しますか？")) {
        state.items = [...DEFAULT_ITEMS];
        saveToCache();
        renderItems();
    }
}

export function clearHistory() {
    if (confirm("履歴を消去しますか？")) {
        state.history = [];
        saveToCache();
        renderHistory();
    }
}

function pickWeightedIndex() {
    if (state.items.length === 0) return -1;
    
    const totalWeight = state.items.reduce((sum, item) => sum + (item.weight || 1), 0);
    let threshold = Math.random() * totalWeight;
    
    for (let i = 0; i < state.items.length; i++) {
        threshold -= (state.items[i].weight || 1);
        if (threshold < 0) return i;
    }
    return state.items.length - 1;
}

export function spin() {
    if (state.spinning || state.items.length === 0) return;
    
    const box = document.getElementById("raffle-box");
    const ticket = document.getElementById("raffle-ticket");
    const ticketLabel = document.getElementById("ticket-label");
    const spinButton = document.getElementById("index-spin-button");
    
    state.spinning = true;
    if (spinButton) {
        spinButton.disabled = true;
        spinButton.textContent = "抽選中...";
    }

    // Hide old ticket
    ticket.classList.remove("showing");
    box.classList.add("shaking");

    const selectedIndex = pickWeightedIndex();
    const selected = state.items[selectedIndex];
    
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
            
            // Add to history
            state.history.unshift({
                ...selected,
                timestamp: new Date().toISOString()
            });
            state.history = state.history.slice(0, 10);
            
            saveToCache();
            renderHistory();
            openResultModal(selected);
        }, 800);
    }, 1500);
}

// --- Modal Logic ---

function ensureResultModal() {
    let modal = document.getElementById("result-modal");
    if (modal) return modal;

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
    modal.addEventListener("click", (event) => {
        if (event.target.closest("[data-close-modal]")) closeResultModal();
    });
    document.body.appendChild(modal);
    return modal;
}

function openResultModal(selected) {
    const modal = ensureResultModal();
    modal.querySelector("#result-modal-title").textContent = selected.label;
    modal.querySelector("#result-modal-chip-label").textContent = selected.label;
    modal.querySelector("#result-modal-swatch").style.background = selected.color || "var(--accent)";
    modal.classList.add("is-open");
    document.body.classList.add("modal-open");
}

function closeResultModal() {
    const modal = document.getElementById("result-modal");
    if (!modal) return;
    modal.classList.remove("is-open");
    document.body.classList.remove("modal-open");
}

// --- Init ---

function init() {
    loadFromCache();
    renderItems();
    renderHistory();
    
    const addButton = document.getElementById("add-item-button");
    if (addButton) addButton.onclick = addItem;

    const resetButton = document.getElementById("reset-items-button");
    if (resetButton) resetButton.onclick = resetItems;
    
    const input = document.getElementById("new-item-label");
    if (input) {
        input.onkeypress = (e) => {
            if (e.key === "Enter") addItem();
        };
    }

    const spinButton = document.getElementById("index-spin-button");
    if (spinButton) spinButton.onclick = spin;
    
    document.addEventListener("keydown", (event) => {
        if (event.key === "Escape") closeResultModal();
    });
}

// すぐに実行（moduleなのでDOM構築を待つ必要はないが、一応DOMContentLoadedも考慮）
if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", init);
} else {
    init();
}
