import { renderHeader } from "./components.js";
import { fetchAppData } from "./api.js";
import { clearCache } from "./cache.js";
import { logoutUser } from "./auth.js";

const COLORS = ["#ef476f", "#ff9f1c", "#ffd166", "#06d6a0", "#118ab2", "#7b2cbf"];

const statusElement = document.getElementById("admin-status");
const rafflesList = document.getElementById("users-list");
const rafflesEmpty = document.getElementById("users-empty");
const raffleModal = document.getElementById("raffle-modal");
const raffleForm = document.getElementById("raffle-form");
const modalItemList = document.getElementById("modal-item-list");

function normalizeWeight(value) {
    const parsed = Number.parseInt(value, 10);
    return Number.isFinite(parsed) && parsed > 0 ? parsed : 1;
}

function setStatus(message, type = "info") {
    if (!statusElement) return;
    statusElement.textContent = message;
    statusElement.dataset.tone = type;
}

function renderRaffles(raffles) {
    if (!rafflesList) return;
    rafflesList.innerHTML = "";

    if (!raffles || raffles.length === 0) {
        if (rafflesEmpty) {
            rafflesEmpty.style.display = "block";
            rafflesEmpty.textContent = "くじ引きはまだありません。";
        }
        return;
    }

    if (rafflesEmpty) rafflesEmpty.style.display = "none";

    const fragment = document.createDocumentFragment();

    raffles.forEach((raffle) => {
        const card = document.createElement("div");
        card.className = "raffle-card";

        const header = document.createElement("div");
        header.className = "raffle-card-header";
        
        const title = document.createElement("strong");
        title.className = "raffle-card-title";
        title.textContent = raffle.title;

        const desc = document.createElement("p");
        desc.className = "raffle-card-desc";
        desc.textContent = raffle.description || "説明はありません。";

        header.appendChild(title);
        header.appendChild(desc);

        const stats = document.createElement("div");
        stats.className = "raffle-card-stats";
        
        const count = document.createElement("span");
        count.innerHTML = `<span class="material-symbols-outlined" style="font-size: 1.1rem;">list_alt</span> ${raffle.items ? raffle.items.length : 0} 項目`;

        const date = document.createElement("span");
        const formattedDate = new Date(raffle.updatedAt).toLocaleDateString();
        date.innerHTML = `<span class="material-symbols-outlined" style="font-size: 1.1rem;">calendar_today</span> 更新: ${formattedDate}`;

        stats.appendChild(count);
        stats.appendChild(date);

        const actions = document.createElement("div");
        actions.className = "raffle-card-actions";

        const playButton = document.createElement("button");
        playButton.className = "btn primary";
        playButton.innerHTML = `<span class="material-symbols-outlined" style="font-size: 1.1rem; margin-right: 4px;">play_arrow</span> 実行`;
        playButton.onclick = () => window.playRaffle(raffle.id);

        const editButton = document.createElement("button");
        editButton.className = "btn";
        editButton.innerHTML = `<span class="material-symbols-outlined" style="font-size: 1.1rem; margin-right: 4px;">edit</span> 編集`;
        editButton.onclick = () => window.openEditModal(raffle);

        const deleteButton = document.createElement("button");
        deleteButton.className = "btn delete-btn";
        deleteButton.innerHTML = `<span class="material-symbols-outlined" style="font-size: 1.1rem;">delete</span>`;
        deleteButton.title = "削除";
        deleteButton.onclick = () => window.deleteRaffle(raffle.id);
        deleteButton.style.flex = "0 0 44px";

        actions.appendChild(playButton);
        actions.appendChild(editButton);
        actions.appendChild(deleteButton);

        card.appendChild(header);
        card.appendChild(stats);
        card.appendChild(actions);
        fragment.appendChild(card);
    });

    rafflesList.appendChild(fragment);
}

export function openCreateModal() {
    if (!raffleModal) return;
    document.getElementById("modal-title").textContent = "くじ引きの作成";
    document.getElementById("edit-id").value = "";
    document.getElementById("modal-raffle-title").value = "";
    document.getElementById("modal-raffle-desc").value = "";
    const preventDuplicatesCheck = document.getElementById("modal-prevent-duplicates");
    if (preventDuplicatesCheck) preventDuplicatesCheck.checked = false;
    
    if (modalItemList) {
        modalItemList.innerHTML = "";
        addItemToModal();
        addItemToModal();
    }
    raffleModal.classList.add("is-open");
    document.body.classList.add("modal-open");
}

window.openEditModal = (raffle) => {
    if (!raffleModal) return;
    document.getElementById("modal-title").textContent = "くじ引きの編集";
    document.getElementById("edit-id").value = raffle.id;
    document.getElementById("modal-raffle-title").value = raffle.title;
    document.getElementById("modal-raffle-desc").value = raffle.description || "";
    const preventDuplicatesCheck = document.getElementById("modal-prevent-duplicates");
    if (preventDuplicatesCheck) preventDuplicatesCheck.checked = !!raffle.preventDuplicates;
    
    if (modalItemList) {
        modalItemList.innerHTML = "";
        if (raffle.items) {
            raffle.items.forEach((item) => addItemToModal(item.label, item.color, item.weight));
        }
    }

    raffleModal.classList.add("is-open");
    document.body.classList.add("modal-open");
};

export function closeRaffleModal() {
    if (raffleModal) raffleModal.classList.remove("is-open");
    document.body.classList.remove("modal-open");
}

export function addItemToModal(label = "", color = "", weight = 1) {
    if (!modalItemList) return;
    const row = document.createElement("div");
    row.className = "modal-item-row";

    const resolvedColor = color || COLORS[modalItemList.children.length % COLORS.length];

    const colorInput = document.createElement("input");
    colorInput.type = "color";
    colorInput.className = "item-color-input";
    colorInput.value = resolvedColor;

    const labelInput = document.createElement("input");
    labelInput.type = "text";
    labelInput.className = "item-label-input";
    labelInput.value = label;
    labelInput.placeholder = "項目名";
    labelInput.required = true;

    const weightInput = document.createElement("input");
    weightInput.type = "number";
    weightInput.className = "item-weight-input";
    weightInput.min = "1";
    weightInput.step = "1";
    weightInput.value = String(normalizeWeight(weight));
    weightInput.title = "比重";

    const removeButton = document.createElement("button");
    removeButton.type = "button";
    removeButton.className = "item-remove-btn";
    removeButton.innerHTML = `<span class="material-symbols-outlined" style="font-size: 1.1rem;">delete</span>`;
    removeButton.onclick = () => {
        row.style.opacity = "0";
        row.style.transform = "translateX(10px)";
        setTimeout(() => row.remove(), 150);
    };

    row.appendChild(colorInput);
    row.appendChild(labelInput);
    row.appendChild(weightInput);
    row.appendChild(removeButton);
    modalItemList.appendChild(row);
}

async function initDashboard() {
    try {
        const { user, raffles } = await fetchAppData();
        const userEmailEl = document.getElementById("user-email");
        if (userEmailEl) userEmailEl.textContent = user.email;
        
        renderRaffles(raffles);
        if (statusElement) statusElement.style.display = "none";
    } catch (error) {
        if (error.code === "UNAUTHORIZED") {
            await logoutUser();
            return;
        }

        setStatus(error.message, "error");
        if (statusElement) statusElement.style.display = "block";
    }
}

async function saveRaffle(event) {
    event.preventDefault();

    if (!modalItemList) return;
    const id = document.getElementById("edit-id").value;
    const items = Array.from(modalItemList.children).map((row) => ({
        label: row.querySelector(".item-label-input").value,
        color: row.querySelector(".item-color-input").value,
        weight: normalizeWeight(row.querySelector(".item-weight-input").value),
    }));

    const payload = {
        id: id || "0",
        title: document.getElementById("modal-raffle-title").value,
        description: document.getElementById("modal-raffle-desc").value,
        preventDuplicates: document.getElementById("modal-prevent-duplicates").checked,
        items,
    };

    try {
        const response = await fetch("/api/dashboard/raffles", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            credentials: "include",
            body: JSON.stringify(payload),
        });

        if (response.status === 401) {
            await logoutUser();
            return;
        }

        if (!response.ok) {
            throw new Error("保存に失敗しました");
        }

        closeRaffleModal();
        clearCache();
        await initDashboard();
    } catch (error) {
        setStatus(error.message, "error");
    }
}

async function init() {
    renderHeader();

    if (raffleForm) {
        raffleForm.onsubmit = saveRaffle;
    }
    
    window.deleteRaffle = async (id) => {
        if (!confirm("このくじ引きを削除しますか？")) return;
        try {
            const res = await fetch(`/api/dashboard/raffles/${id}`, {
                method: "DELETE",
                credentials: "include",
            });
            if (res.status === 401) {
                await logoutUser();
                return;
            }
            if (!res.ok) throw new Error("削除に失敗しました");
            clearCache();
            await initDashboard();
        } catch (error) {
            setStatus(error.message, "error");
        }
    };

    window.playRaffle = (id) => {
        window.location.href = `/raffle?id=${id}`;
    };

    await initDashboard();
}

init();
