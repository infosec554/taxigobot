const tg = window.Telegram.WebApp;
const monthNames = ["Yanvar", "Fevral", "Mart", "Aprel", "May", "Iyun", "Iyul", "Avgust", "Sentabr", "Oktabr", "Noyabr", "Dekabr"];
const weekdayNames = ["Yakshanba", "Dushanba", "Seshanba", "Chorshanba", "Payshanba", "Juma", "Shanba"];

let currentDate = new Date();
let allOrders = [];
let selectedOrder = null;

// UI Elements
const grid = document.getElementById('calendarGrid');
const monthDisplay = document.getElementById('monthDisplay');
const modal = document.getElementById('orderModal');

async function init() {
    tg.expand();
    tg.ready();
    await fetchOrders();
    render();
}

async function fetchOrders() {
    try {
        const res = await fetch('/api/orders/active');
        allOrders = await res.json();
    } catch (e) { console.error("API error", e); }
}

function render() {
    monthDisplay.innerText = `${monthNames[currentDate.getMonth()]} ${currentDate.getFullYear()}`;

    // Clear previous days (keep weekdays)
    const weekdayHeaders = Array.from(grid.querySelectorAll('.weekday'));
    grid.innerHTML = '';
    weekdayHeaders.forEach(h => grid.appendChild(h));

    const year = currentDate.getFullYear();
    const month = currentDate.getMonth();
    const firstDay = new Date(year, month, 1).getDay();
    const daysInMonth = new Date(year, month + 1, 0).getDate();

    // Adjust for Monday start (0=Mon, 6=Sun)
    let offset = firstDay === 0 ? 6 : firstDay - 1;

    for (let i = 0; i < offset; i++) {
        grid.appendChild(document.createElement('div'));
    }

    const today = new Date();

    for (let d = 1; d <= daysInMonth; d++) {
        const cell = document.createElement('div');
        cell.className = 'day-cell';
        if (d === today.getDate() && month === today.getMonth() && year === today.getFullYear()) {
            cell.classList.add('today');
        }

        cell.innerHTML = `
            <div class="day-header"><div class="day-number">${d}</div></div>
            <div class="events-container"></div>
        `;

        const dateStr = `${year}-${String(month + 1).padStart(2, '0')}-${String(d).padStart(2, '0')}`;
        const dayOrders = allOrders.filter(o => o.PickupTime && o.PickupTime.startsWith(dateStr));
        const container = cell.querySelector('.events-container');

        dayOrders.forEach(o => {
            const time = o.PickupTime.split('T')[1].substring(0, 5);
            const ev = document.createElement('div');
            ev.className = `event-item ${o.Status === 'taken' ? 'taken' : ''}`;
            ev.innerText = `${time} ${o.ToLocationName}`;
            ev.onclick = (e) => {
                e.stopPropagation();
                showDetail(o);
            };
            container.appendChild(ev);
        });

        // Click on the cell also shows the first order or a list
        cell.onclick = () => {
            if (dayOrders.length > 0) {
                showDetail(dayOrders[0]);
            }
        };

        grid.appendChild(cell);
    }
}

function showDetail(o) {
    selectedOrder = o;
    const date = new Date(o.PickupTime);
    document.getElementById('modalDate').innerText = `${date.getDate()}-${monthNames[date.getMonth()]}, ${weekdayNames[date.getDay()]}`;
    document.getElementById('modalTitle').innerText = `${o.FromLocationName} ➞ ${o.ToLocationName}`;
    document.getElementById('modalTime').innerText = `🕒 ${o.PickupTime.split('T')[1].substring(0, 5)}`;
    document.getElementById('modalPax').innerText = `👥 ${o.Passengers} kishi`;
    document.getElementById('modalPrice').innerText = `💰 ${o.Price.toLocaleString()} ${o.Currency || "RUB"}`;

    modal.classList.remove('hidden');
}

document.getElementById('takeBtn').onclick = () => {
    if (selectedOrder) {
        tg.sendData(JSON.stringify({ action: 'take_order', order_id: selectedOrder.ID }));
        modal.classList.add('hidden');
    }
};

document.getElementById('prevBtn').onclick = () => { currentDate.setMonth(currentDate.getMonth() - 1); render(); };
document.getElementById('nextBtn').onclick = () => { currentDate.setMonth(currentDate.getMonth() + 1); render(); };
document.getElementById('closeModal').onclick = () => modal.classList.add('hidden');
document.getElementById('cancelModal').onclick = () => modal.classList.add('hidden');

init();
