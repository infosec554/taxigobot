const tg = window.Telegram.WebApp;
tg.expand();
tg.ready();

let currentDate = new Date();
let allOrders = [];

const monthNames = ["Yanvar", "Fevral", "Mart", "Aprel", "May", "Iyun", "Iyul", "Avgust", "Sentabr", "Oktabr", "Noyabr", "Dekabr"];

// DOM Elements
const monthDisplay = document.getElementById('currentMonthYear');
const calendarGrid = document.getElementById('calendarGrid');
const orderList = document.getElementById('orderList');
const gridBtn = document.getElementById('gridBtn');
const listBtn = document.getElementById('listBtn');
const modal = document.getElementById('orderModal');
const modalBody = document.getElementById('modalBody');

// Initialize
async function init() {
    await fetchOrders();
    render();
}

async function fetchOrders() {
    try {
        const response = await fetch('/api/orders/active');
        allOrders = await response.json();
        console.log("Orders fetched:", allOrders);
    } catch (error) {
        console.error("Error fetching orders:", error);
    }
}

function render() {
    monthDisplay.innerText = `${monthNames[currentDate.getMonth()]} ${currentDate.getFullYear()}`;
    renderGrid();
    renderList();
}

function renderGrid() {
    // Clear previous cells except headers
    const headers = calendarGrid.querySelectorAll('.weekday');
    calendarGrid.innerHTML = '';
    headers.forEach(h => calendarGrid.appendChild(h));

    const year = currentDate.getFullYear();
    const month = currentDate.getMonth();

    const firstDay = new Date(year, month, 1).getDay();
    const daysInMonth = new Date(year, month + 1, 0).getDate();

    // Adjust first day (Monday = 0)
    let startDay = firstDay === 0 ? 6 : firstDay - 1;

    // Empty cells
    for (let i = 0; i < startDay; i++) {
        calendarGrid.appendChild(document.createElement('div'));
    }

    // Days
    for (let d = 1; d <= daysInMonth; d++) {
        const cell = document.createElement('div');
        cell.className = 'day-cell';

        const today = new Date();
        if (d === today.getDate() && month === today.getMonth() && year === today.getFullYear()) {
            cell.classList.add('today');
        }

        cell.innerHTML = `
            <div class="day-number">${d}</div>
            <div class="order-snippets"></div>
        `;

        // Check for orders on this day
        const dayStr = `${year}-${String(month + 1).padStart(2, '0')}-${String(d).padStart(2, '0')}`;
        const dayOrders = allOrders.filter(o => o.PickupTime && o.PickupTime.startsWith(dayStr));

        const snippetContainer = cell.querySelector('.order-snippets');
        dayOrders.slice(0, 3).forEach(o => {
            const time = o.PickupTime.split('T')[1].substring(0, 5);
            const snip = document.createElement('div');
            snip.className = 'snippet';
            snip.innerText = `${time} ${o.ToLocationName}`;
            snippetContainer.appendChild(snip);
        });

        if (dayOrders.length > 3) {
            const more = document.createElement('div');
            more.className = 'snippet';
            more.style.background = 'transparent';
            more.innerText = `+${dayOrders.length - 3} ta...`;
            snippetContainer.appendChild(more);
        }

        cell.onclick = () => showDayDetails(dayStr, dayOrders);
        calendarGrid.appendChild(cell);
    }
}

function renderList() {
    orderList.innerHTML = '';

    // Group orders by date
    const grouped = {};
    allOrders.forEach(o => {
        if (!o.PickupTime) return;
        const date = o.PickupTime.split('T')[0];
        if (!grouped[date]) grouped[date] = [];
        grouped[date].push(o);
    });

    const sortedDates = Object.keys(grouped).sort();

    sortedDates.forEach(date => {
        const groupDiv = document.createElement('div');
        groupDiv.className = 'date-group';

        const d = new Date(date);
        groupDiv.innerHTML = `<h3>${d.getDate()} ${monthNames[d.getMonth()]}</h3>`;

        grouped[date].forEach(o => {
            const time = o.PickupTime.split('T')[1].substring(0, 5);
            const card = document.createElement('div');
            card.className = 'order-card';
            card.innerHTML = `
                <div class="order-time">${time}</div>
                <div class="order-info">
                    <div class="order-route">${o.FromLocationName} ➞ ${o.ToLocationName}</div>
                    <div class="order-meta">${o.Passengers} kishi • ${o.Price} ${o.Currency}</div>
                </div>
            `;
            card.onclick = () => showOrderDetails(o);
            groupDiv.appendChild(card);
        });
        orderList.appendChild(groupDiv);
    });

    if (allOrders.length === 0) {
        orderList.innerHTML = '<div style="text-align:center; padding: 40px; color: var(--text-secondary)">Hozircha zakazlar yo\'q.</div>';
    }
}

function showDayDetails(dayStr, orders) {
    if (orders.length === 0) return;
    // For now, if multiple orders, we could show a list, but let's just show the first for simplicity
    // in a real app, you'd show a list of those specific orders.
    showOrderDetails(orders[0]);
}

function showOrderDetails(o) {
    const time = o.PickupTime.split('T')[1].substring(0, 5);
    const date = new Date(o.PickupTime.split('T')[0]);

    modalBody.innerHTML = `
        <div class="modal-route">${o.FromLocationName} ➞ ${o.ToLocationName}</div>
        <div class="detail-item">
            <span class="detail-label">Sana:</span>
            <span>${date.getDate()} ${monthNames[date.getMonth()]}</span>
        </div>
        <div class="detail-item">
            <span class="detail-label">Vaqt:</span>
            <span>${time}</span>
        </div>
        <div class="detail-item">
            <span class="detail-label">Yo'lovchilar:</span>
            <span>${o.Passengers} kishi</span>
        </div>
        <div class="detail-item">
            <span class="detail-label">Narx:</span>
            <span style="color: var(--accent-color); font-weight: 700;">${o.Price} ${o.Currency}</span>
        </div>
        <button class="take-btn" onclick="takeOrder(${o.ID})">ZAKAZNI OLISH</button>
    `;
    modal.classList.remove('hidden');
}

window.takeOrder = function (id) {
    tg.sendData(JSON.stringify({ action: 'take_order', order_id: id }));
    modal.classList.add('hidden');
};

// Event Listeners
document.getElementById('prevMonth').onclick = () => {
    currentDate.setMonth(currentDate.getMonth() - 1);
    render();
};
document.getElementById('nextMonth').onclick = () => {
    currentDate.setMonth(currentDate.getMonth() + 1);
    render();
};

gridBtn.onclick = () => {
    gridBtn.classList.add('active');
    listBtn.classList.remove('active');
    calendarGrid.classList.remove('hidden');
    orderList.classList.add('hidden');
};

listBtn.onclick = () => {
    listBtn.classList.add('active');
    gridBtn.classList.remove('active');
    orderList.classList.remove('hidden');
    calendarGrid.classList.add('hidden');
};

document.querySelector('.close-modal').onclick = () => {
    modal.classList.add('hidden');
};

init();
