const API_BASE = '';

// Утилиты
function showToast(message, type) {
    const container = document.getElementById('toastContainer');
    if (!container) return;
    const bg = type === 'error' ? 'bg-danger' : 'bg-success';
    const toastEl = document.createElement('div');
    toastEl.className = `toast`;
    toastEl.setAttribute('role', 'alert');
    toastEl.innerHTML = `
        <div class="toast-header text-white ${bg} border-0">
            <strong class="me-auto">${type === 'error' ? 'Ошибка' : 'Успех'}</strong>
            <button type="button" class="btn-close btn-close-white" data-bs-dismiss="toast" aria-label="Закрыть"></button>
        </div>
        <div class="toast-body">${escapeHtml(message)}</div>`;
    container.appendChild(toastEl);
    const toast = new bootstrap.Toast(toastEl, { delay: type === 'error' ? 5000 : 3000 });
    toast.show();
    toastEl.addEventListener('hidden.bs.toast', () => toastEl.remove());
}

function showError(message) {
    showToast(message, 'error');
}

function showSuccess(message) {
    showToast(message, 'success');
}

async function apiCall(endpoint, options = {}) {
    try {
        const url = API_BASE + endpoint;
        console.log('API Call:', url, options.method || 'GET');
        
        const response = await fetch(url, {
            headers: {
                'Content-Type': 'application/json',
                ...options.headers
            },
            ...options
        });
        
        console.log('API Response status:', response.status, response.statusText);
        
        if (!response.ok) {
            const errorText = await response.text();
            console.error('API Error response:', errorText);
            throw new Error(errorText || `HTTP ${response.status}`);
        }
        
        const contentType = response.headers.get('content-type');
        if (contentType && contentType.includes('application/json')) {
            const data = await response.json();
            console.log('API Response data:', data);
            return data;
        }
        
        console.log('API Response: no JSON content');
        return null;
    } catch (error) {
        console.error('API Error:', error);
        throw error;
    }
}

// Chart.js defaults — read colors from Bootstrap CSS variables at runtime
const _bs = (v) => getComputedStyle(document.documentElement).getPropertyValue(v).trim();
const _bsBodyColor = _bs('--bs-body-color') || '#212529';
const _bsBorderColor = _bs('--bs-border-color') || '#dee2e6';
const _bsSecondaryColor = _bs('--bs-secondary-color') || '#6c757d';
Chart.defaults.color = _bsBodyColor;
Chart.defaults.borderColor = _bsBorderColor;
Chart.defaults.plugins.legend.labels.color = _bsSecondaryColor;
Chart.defaults.scale.grid = { ...Chart.defaults.scale.grid, color: _bsBorderColor };

// Кэш проверок по домену (используется для обновления графиков без лишних запросов)
const domainChecksCache = new Map();

// Загрузка доменов
async function loadDomains() {
    const listEl = document.getElementById('domainsList');
    const currentDomains = new Set();

    try {
        const domains = await apiCall('/domains');

        if (!domains || !Array.isArray(domains)) {
            if (listEl.innerHTML.includes('Загрузка') || listEl.querySelector('.spinner-border')) {
                listEl.innerHTML = '<div class="alert alert-danger">Ошибка: неверный формат ответа от сервера</div>';
            }
            return;
        }

        if (domains.length === 0) {
            domainCharts.forEach((chart) => chart.destroy());
            domainCharts.clear();
            checkCharts.forEach((chart) => chart.destroy());
            checkCharts.clear();
            domainChecksCache.clear();
            listEl.innerHTML = '<p class="text-center text-muted py-4 mb-0">Нет доменов для мониторинга</p>';
            return;
        }

        const loadingEl = listEl.querySelector('.spinner-border, .text-muted');
        if (loadingEl && loadingEl.textContent.includes('Загрузка')) listEl.innerHTML = '';

        const existingDomainIds = new Set(domains.map(d => d.id));

        // Удаляем графики и элементы для доменов, которых больше нет
        domainCharts.forEach((chart, domainId) => {
            if (!existingDomainIds.has(domainId)) {
                chart.destroy();
                domainCharts.delete(domainId);
            }
        });
        listEl.querySelectorAll('[id^="domain-"]').forEach(el => {
            const domainId = parseInt(el.id.replace('domain-', ''));
            if (!existingDomainIds.has(domainId)) {
                domainChecksCache.delete(domainId);
                el.remove();
            }
        });

        // Создаём только новые элементы доменов, существующие не трогаем
        for (const domain of domains) {
            currentDomains.add(domain.id);
            if (!document.getElementById(`domain-${domain.id}`)) {
                const domainEl = createDomainElement(domain);
                listEl.appendChild(domainEl);
                await loadChecksForDomain(domain.id);
            }
        }

        // Обновляем графики всех доменов параллельно
        await updateAllDomainCharts(domains);

    } catch (error) {
        if (listEl.innerHTML.includes('Загрузка') || listEl.querySelector('.spinner-border')) {
            listEl.innerHTML = `<div class="alert alert-danger">Ошибка загрузки: ${escapeHtml(error.message)}</div>`;
        }
    }
}

// Параллельное обновление графиков всех доменов
async function updateAllDomainCharts(domains) {
    const to = new Date();
    to.setSeconds(0, 0);
    const from = new Date(to.getTime() - 10 * 60 * 1000);
    const fromStr = from.toISOString();
    const toStr = to.toISOString();

    await Promise.all(domains.map(async (domain) => {
        try {
            const ctx = document.getElementById(`domainChart-${domain.id}`);
            if (!ctx) return;

            // Используем кэш проверок или загружаем
            let checks = domainChecksCache.get(domain.id);
            if (!checks) {
                checks = await apiCall(`/domains/${domain.id}/checks`);
                if (checks && Array.isArray(checks)) {
                    domainChecksCache.set(domain.id, checks);
                }
            }
            if (!checks || !Array.isArray(checks) || checks.length === 0) return;

            // Загружаем результаты всех проверок параллельно
            const resultsArrays = await Promise.all(
                checks.map(check =>
                    apiCall(`/checks/${check.id}/results?from=${encodeURIComponent(fromStr)}&to=${encodeURIComponent(toStr)}&page=1&page_size=100`)
                        .then(r => (r && r.results && Array.isArray(r.results)) ? r.results : [])
                        .catch(() => [])
                )
            );
            const allResults = resultsArrays.flat();

            const aggregatedData = aggregateResultsByMinute(allResults, false);
            const labels = aggregatedData.map(item => {
                const date = new Date(item.timestamp.replace(' ', 'T') + 'Z');
                return date.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' });
            });
            const successData = aggregatedData.map(item => item.success_count || 0);
            const failureData = aggregatedData.map(item => item.failure_count || 0);
            const latencyData = aggregatedData.map(item => item.avg_latency || 0);

            if (domainCharts.has(domain.id)) {
                const chart = domainCharts.get(domain.id);
                if (chart && chart.canvas && chart.canvas.parentNode) {
                    chart.data.labels = labels;
                    chart.data.datasets[0].data = successData;
                    chart.data.datasets[1].data = failureData;
                    chart.data.datasets[2].data = latencyData;
                    chart.update('none');
                    return;
                }
                domainCharts.delete(domain.id);
            }

            createDomainChartInstance(ctx, domain.id, labels, successData, failureData, latencyData);
        } catch (error) {
            console.error('Error updating chart for domain', domain.id, error);
        }
    }));
}

function createDomainElement(domain) {
    const div = document.createElement('div');
    div.className = 'card';
    div.id = `domain-${domain.id}`;
    div.innerHTML = `
        <div class="card-body">
            <div class="d-flex flex-wrap justify-content-between align-items-center gap-2 mb-3">
                <h5 class="card-title mb-0">${escapeHtml(domain.name)}</h5>
                <div class="btn-group btn-group-sm">
                    <button class="btn btn-primary" onclick="openCheckModal(${domain.id})">+ Проверка</button>
                    <button class="btn btn-danger" onclick="deleteDomain(${domain.id})">Удалить</button>
                </div>
            </div>
            <div class="position-relative w-100 mb-3" style="height: 200px; cursor: pointer;" onclick="viewDomainResults(${domain.id})">
                <canvas id="domainChart-${domain.id}"></canvas>
            </div>
            <div id="checks-${domain.id}">
                <p class="text-center text-muted py-3 mb-0"><span class="spinner-border spinner-border-sm me-2" role="status"></span>Загрузка проверок...</p>
            </div>
        </div>
    `;
    return div;
}

async function loadChecksForDomain(domainId) {
    const checksEl = document.getElementById(`checks-${domainId}`);
    if (!checksEl) return;

    try {
        const checks = await apiCall(`/domains/${domainId}/checks`);

        if (!checks || !Array.isArray(checks)) {
            checksEl.innerHTML = '<div class="alert alert-danger">Ошибка: неверный формат ответа от сервера</div>';
            return;
        }

        domainChecksCache.set(domainId, checks);

        if (checks.length === 0) {
            checksEl.innerHTML = '<p class="text-center text-muted py-2 mb-0">Нет проверок</p>';
            return;
        }

        const newCheckIds = new Set(checks.map(c => c.id));
        const existingCheckIds = new Set();

        checksEl.querySelectorAll('[id^="check-"]').forEach(el => {
            const checkId = parseInt(el.id.replace('check-', ''));
            if (!newCheckIds.has(checkId)) {
                if (checkCharts.has(checkId)) {
                    checkCharts.get(checkId).destroy();
                    checkCharts.delete(checkId);
                }
                el.remove();
            } else {
                existingCheckIds.add(checkId);
            }
        });

        checksEl.querySelectorAll(':scope > p, :scope > .alert').forEach(el => el.remove());

        // Добавляем/обновляем элементы проверок
        for (const check of checks) {
            const existingEl = document.getElementById(`check-${check.id}`);
            if (existingEl) {
                // Обновляем только текстовое содержимое (статус, детали) без пересоздания DOM
                updateCheckElement(existingEl, check);
            } else {
                const checkEl = createCheckElement(check);
                checksEl.appendChild(checkEl);
            }
        }
    } catch (error) {
        // Не затираем содержимое при ошибке обновления (чтобы не скакать)
        console.error(`Ошибка загрузки проверок для домена ${domainId}:`, error);
    }
}

// Обновление содержимого check-item без пересоздания DOM
function updateCheckElement(el, check) {
    const statusEl = el.querySelector('.badge[data-status]');
    if (statusEl) {
        statusEl.className = `badge ${check.enabled ? 'bg-success' : 'bg-secondary'}`;
        statusEl.textContent = check.enabled ? 'Включена' : 'Отключена';
    }

    const detailsEl = el.querySelector('.small.text-muted');
    if (detailsEl) {
        let details = `Интервал: ${check.interval_seconds || 0}с`;
        if (check.params) {
            if (check.params.path) details += ` | Путь: ${check.params.path}`;
            if (check.params.port) details += ` | Порт: ${check.params.port}`;
        }
        if (check.realtime_mode) details += ` | Реальное время`;
        detailsEl.textContent = details;
    }
}

function createCheckElement(check) {
    const div = document.createElement('div');
    div.className = 'card mb-2';
    div.id = `check-${check.id}`;
    div.setAttribute('data-check-type', (check.type || 'unknown').toLowerCase());

    let details = `Интервал: ${check.interval_seconds || 0}с`;
    if (check.params) {
        if (check.params.path) details += ` | Путь: ${check.params.path}`;
        if (check.params.port) details += ` | Порт: ${check.params.port}`;
    }
    if (check.realtime_mode) details += ` | Реальное время`;

    const checkType = (check.type || 'unknown').toLowerCase();
    const typeBadge = { http: 'bg-info', icmp: 'bg-primary', tcp: 'bg-warning text-dark', udp: 'bg-success', tls: 'bg-danger' }[checkType] || 'bg-secondary';
    const statusBadge = check.enabled ? 'bg-success' : 'bg-secondary';

    div.innerHTML = `
        <div class="card-body py-2">
            <div class="d-flex flex-wrap justify-content-between align-items-start gap-2">
                <div>
                    <span class="badge ${typeBadge} me-1">${checkType.toUpperCase()}</span>
                    <span class="badge ${statusBadge}" data-status>${check.enabled ? 'Включена' : 'Отключена'}</span>
                    <div class="small text-muted mt-1">${details}</div>
                </div>
                <div class="btn-group btn-group-sm">
                    <button class="btn btn-primary" onclick="viewCheckResults(${check.id})">Результаты</button>
                    <button class="btn btn-outline-dark" onclick="editCheck(${check.id})" title="Редактировать">⚙️</button>
                    ${check.enabled
                        ? `<button class="btn btn-outline-dark" onclick="toggleCheck(${check.id}, false)">Отключить</button>`
                        : `<button class="btn btn-success" onclick="toggleCheck(${check.id}, true)">Включить</button>`
                    }
                    <button class="btn btn-danger" onclick="deleteCheck(${check.id})">Удалить</button>
                </div>
            </div>
        </div>
    `;
    return div;
}

// Добавление домена
document.getElementById('addDomainForm').addEventListener('submit', async (e) => {
    e.preventDefault();
    const name = document.getElementById('domainName').value.trim();
    
    if (!name) {
        showError('Введите имя домена');
        return;
    }
    
    try {
        await apiCall('/domains', {
            method: 'POST',
            body: JSON.stringify({ name })
        });
        
        document.getElementById('domainName').value = '';
        showSuccess('Домен добавлен');
        await loadDomains();
    } catch (error) {
        showError(`Ошибка добавления домена: ${error.message}`);
    }
});

// Удаление домена
async function deleteDomain(id) {
    if (!confirm('Удалить домен и все его проверки?')) return;
    
    try {
        await apiCall(`/domains/${id}`, { method: 'DELETE' });
        showSuccess('Домен удален');
        await loadDomains();
    } catch (error) {
        showError(`Ошибка удаления: ${error.message}`);
    }
}

// Модальное окно для создания проверки
let checkTypeHandlerAdded = false;

function openCheckModal(domainId) {
    document.getElementById('checkDomainId').value = domainId;
    const modal = bootstrap.Modal.getOrCreateInstance(document.getElementById('checkModal'));
    modal.show();
    
    // Показываем/скрываем поля в зависимости от типа проверки
    if (!checkTypeHandlerAdded) {
        document.getElementById('checkType').addEventListener('change', updateCheckForm);
        checkTypeHandlerAdded = true;
    }
    updateCheckForm();
    
    // Сбрасываем форму
    document.getElementById('addCheckForm').reset();
    document.getElementById('checkIntervalType').value = 'minute';
    document.getElementById('checkInterval').value = 1;
    document.getElementById('checkScheme').value = 'https';
    document.getElementById('checkMethod').value = 'GET';
}

function updateCheckForm() {
    const type = document.getElementById('checkType').value;
    const httpParams = document.getElementById('httpParams');
    const portParams = document.getElementById('portParams');
    const tcpParams = document.getElementById('tcpParams');
    const udpParams = document.getElementById('udpParams');
    const portInput = document.getElementById('checkPort');

    const needsPort = type === 'tcp' || type === 'udp' || type === 'tls';
    httpParams.style.display = type === 'http' ? 'block' : 'none';
    portParams.style.display = needsPort ? 'block' : 'none';
    tcpParams.style.display = type === 'tcp' ? 'block' : 'none';
    udpParams.style.display = type === 'udp' ? 'block' : 'none';
    if (portInput) portInput.required = needsPort;
}

function updateEditCheckForm() {
    const type = document.getElementById('editCheckType').value;
    const httpParams = document.getElementById('editHttpParams');
    const portParams = document.getElementById('editPortParams');
    const tcpParams = document.getElementById('editTcpParams');
    const udpParams = document.getElementById('editUdpParams');
    const portInput = document.getElementById('editCheckPort');

    const needsPort = type === 'tcp' || type === 'udp' || type === 'tls';
    httpParams.style.display = type === 'http' ? 'block' : 'none';
    portParams.style.display = needsPort ? 'block' : 'none';
    tcpParams.style.display = type === 'tcp' ? 'block' : 'none';
    udpParams.style.display = type === 'udp' ? 'block' : 'none';
    if (portInput) portInput.required = needsPort;
}

function convertIntervalToSeconds(intervalType, intervalValue) {
    const value = parseInt(intervalValue) || 1;
    switch(intervalType) {
        case 'second': return value;
        case 'minute': return value * 60;
        case 'hour': return value * 60 * 60;
        case 'day': return value * 24 * 60 * 60;
        default: return value * 60;
    }
}

document.getElementById('addCheckForm').addEventListener('submit', async (e) => {
    e.preventDefault();
    
    const domainId = parseInt(document.getElementById('checkDomainId').value);
    const type = document.getElementById('checkType').value;
    const intervalType = document.getElementById('checkIntervalType').value;
    const intervalValue = parseInt(document.getElementById('checkInterval').value) || 1;
    const intervalSeconds = convertIntervalToSeconds(intervalType, intervalValue);
    const timeout = parseInt(document.getElementById('checkTimeout').value) || 5000;
    const realtime = document.getElementById('checkRealtime').checked;
    const rateLimit = parseInt(document.getElementById('checkRateLimit').value) || 0;
    
    const params = {};
    if (type === 'http') {
        params.path = document.getElementById('checkPath').value || '/';
        params.scheme = document.getElementById('checkScheme').value || 'https';
        params.method = document.getElementById('checkMethod').value || 'GET';
        const body = document.getElementById('checkBody').value;
        if (body) params.body = body;
    }
    if (type === 'tcp' || type === 'udp' || type === 'tls') {
        const port = parseInt(document.getElementById('checkPort').value);
        if (!port || port < 1 || port > 65535) {
            showError('Укажите корректный порт (1-65535)');
            return;
        }
        params.port = port;
    }
    if (type === 'tcp') {
        const tcpPayload = document.getElementById('checkTcpPayload').value;
        if (tcpPayload) params.payload = tcpPayload;
    }
    if (type === 'udp') {
        const payload = document.getElementById('checkPayload').value;
        if (payload) params.payload = payload;
    }
    if (timeout > 0) {
        params.timeout_ms = timeout;
    }
    
    try {
        await apiCall(`/domains/${domainId}/checks`, {
            method: 'POST',
            body: JSON.stringify({
                type,
                interval_seconds: intervalSeconds,
                params,
                realtime_mode: realtime,
                rate_limit_per_minute: rateLimit
            })
        });
        
        const m = bootstrap.Modal.getInstance(document.getElementById('checkModal'));
        if (m) m.hide();
        document.getElementById('addCheckForm').reset();
        showSuccess('Проверка создана');
        await loadChecksForDomain(domainId);
    } catch (error) {
        showError(`Ошибка создания проверки: ${error.message}`);
    }
});

// Редактирование проверки
async function editCheck(checkId) {
    try {
        // Загружаем информацию о проверке
        const checks = await apiCall('/checks');
        const check = checks.find(c => c.id === checkId);
        
        if (!check) {
            showError('Проверка не найдена');
            return;
        }
        
        // Заполняем форму редактирования
        document.getElementById('editCheckId').value = check.id;
        document.getElementById('editCheckDomainId').value = check.domain_id;
        document.getElementById('editCheckType').value = check.type;
        
        // Конвертируем интервал обратно в тип и значение
        const intervalSeconds = check.interval_seconds || 60;
        let intervalType = 'minute';
        let intervalValue = 1;
        if (intervalSeconds < 60) {
            intervalType = 'second';
            intervalValue = intervalSeconds;
        } else if (intervalSeconds < 3600) {
            intervalType = 'minute';
            intervalValue = Math.floor(intervalSeconds / 60);
        } else if (intervalSeconds < 86400) {
            intervalType = 'hour';
            intervalValue = Math.floor(intervalSeconds / 3600);
        } else {
            intervalType = 'day';
            intervalValue = Math.floor(intervalSeconds / 86400);
        }
        document.getElementById('editCheckIntervalType').value = intervalType;
        document.getElementById('editCheckInterval').value = intervalValue;
        
        // Заполняем параметры
        if (check.params) {
            if (check.type === 'http') {
                document.getElementById('editCheckScheme').value = check.params.scheme || 'https';
                document.getElementById('editCheckPath').value = check.params.path || '/';
                document.getElementById('editCheckMethod').value = check.params.method || 'GET';
                document.getElementById('editCheckBody').value = check.params.body || '';
            }
            if (check.type === 'tcp' || check.type === 'udp' || check.type === 'tls') {
                document.getElementById('editCheckPort').value = check.params.port || '';
            }
            if (check.type === 'tcp') {
                document.getElementById('editCheckTcpPayload').value = check.params.payload || '';
            }
            if (check.type === 'udp') {
                document.getElementById('editCheckPayload').value = check.params.payload || '';
            }
            document.getElementById('editCheckTimeout').value = check.params.timeout_ms || 5000;
        }
        
        document.getElementById('editCheckRealtime').checked = check.realtime_mode || false;
        document.getElementById('editCheckRateLimit').value = check.rate_limit_per_minute || 0;
        
        // Обновляем видимость полей
        updateEditCheckForm();
        
        bootstrap.Modal.getOrCreateInstance(document.getElementById('editCheckModal')).show();
    } catch (error) {
        showError(`Ошибка загрузки проверки: ${error.message}`);
    }
}

// Обработчик изменения типа проверки в форме редактирования
if (!document.getElementById('editCheckType').hasAttribute('data-handler-added')) {
    document.getElementById('editCheckType').addEventListener('change', updateEditCheckForm);
    document.getElementById('editCheckType').setAttribute('data-handler-added', 'true');
}

// Обработчик отправки формы редактирования
document.getElementById('editCheckForm').addEventListener('submit', async (e) => {
    e.preventDefault();
    
    const checkId = parseInt(document.getElementById('editCheckId').value);
    const domainId = parseInt(document.getElementById('editCheckDomainId').value);
    const type = document.getElementById('editCheckType').value;
    const intervalType = document.getElementById('editCheckIntervalType').value;
    const intervalValue = parseInt(document.getElementById('editCheckInterval').value) || 1;
    const intervalSeconds = convertIntervalToSeconds(intervalType, intervalValue);
    const timeout = parseInt(document.getElementById('editCheckTimeout').value) || 5000;
    const realtime = document.getElementById('editCheckRealtime').checked;
    const rateLimit = parseInt(document.getElementById('editCheckRateLimit').value) || 0;
    
    const params = {};
    if (type === 'http') {
        params.path = document.getElementById('editCheckPath').value || '/';
        params.scheme = document.getElementById('editCheckScheme').value || 'https';
        params.method = document.getElementById('editCheckMethod').value || 'GET';
        const body = document.getElementById('editCheckBody').value;
        if (body) params.body = body;
    }
    if (type === 'tcp' || type === 'udp' || type === 'tls') {
        const port = parseInt(document.getElementById('editCheckPort').value);
        if (!port || port < 1 || port > 65535) {
            showError('Укажите корректный порт (1-65535)');
            return;
        }
        params.port = port;
    }
    if (type === 'tcp') {
        const tcpPayload = document.getElementById('editCheckTcpPayload').value;
        if (tcpPayload) params.payload = tcpPayload;
    }
    if (type === 'udp') {
        const payload = document.getElementById('editCheckPayload').value;
        if (payload) params.payload = payload;
    }
    if (timeout > 0) {
        params.timeout_ms = timeout;
    }
    
    try {
        await apiCall(`/checks/${checkId}`, {
            method: 'PUT',
            body: JSON.stringify({
                type,
                interval_seconds: intervalSeconds,
                params,
                realtime_mode: realtime,
                rate_limit_per_minute: rateLimit
            })
        });
        
        const em = bootstrap.Modal.getInstance(document.getElementById('editCheckModal'));
        if (em) em.hide();
        showSuccess('Проверка обновлена');
        await loadChecksForDomain(domainId);
    } catch (error) {
        showError(`Ошибка обновления проверки: ${error.message}`);
    }
});

// Обработчик кнопки удаления в форме редактирования
document.getElementById('deleteCheckBtn').addEventListener('click', async () => {
    const checkId = parseInt(document.getElementById('editCheckId').value);
    const domainId = parseInt(document.getElementById('editCheckDomainId').value);
    
    if (!confirm('Удалить проверку?')) return;
    
    try {
        await apiCall(`/checks/${checkId}`, { method: 'DELETE' });
        const em = bootstrap.Modal.getInstance(document.getElementById('editCheckModal'));
        if (em) em.hide();
        showSuccess('Проверка удалена');
        
        // Удаляем график из хранилища
        if (checkCharts.has(checkId)) {
            checkCharts.get(checkId).destroy();
            checkCharts.delete(checkId);
        }
        
        await loadChecksForDomain(domainId);
    } catch (error) {
        showError(`Ошибка удаления: ${error.message}`);
    }
});

// Удаление проверки
async function deleteCheck(id) {
    if (!confirm('Удалить проверку?')) return;
    
    try {
        await apiCall(`/checks/${id}`, { method: 'DELETE' });
        showSuccess('Проверка удалена');
        
        // Удаляем график из хранилища
        if (checkCharts.has(id)) {
            checkCharts.get(id).destroy();
            checkCharts.delete(id);
        }
        
        // Инвалидируем кэш
        domainChecksCache.clear();

        const checkEl = document.getElementById(`check-${id}`);
        if (checkEl) {
            checkEl.remove();
        } else {
            await loadDomains();
        }
    } catch (error) {
        showError(`Ошибка удаления: ${error.message}`);
    }
}

// Включение/отключение проверки
async function toggleCheck(id, enabled) {
    try {
        const endpoint = enabled ? `/checks/${id}/enable` : `/checks/${id}/disable`;
        await apiCall(endpoint, { method: 'POST' });

        const el = document.getElementById(`check-${id}`);
        if (el) {
            const statusEl = el.querySelector('.badge[data-status]');
            if (statusEl) {
                statusEl.className = `badge ${enabled ? 'bg-success' : 'bg-secondary'}`;
                statusEl.textContent = enabled ? 'Включена' : 'Отключена';
            }
            const toggleBtn = el.querySelector('.btn-group button:nth-child(3)');
            if (toggleBtn) {
                if (enabled) {
                    toggleBtn.className = 'btn btn-outline-dark';
                    toggleBtn.textContent = 'Отключить';
                    toggleBtn.setAttribute('onclick', `toggleCheck(${id}, false)`);
                } else {
                    toggleBtn.className = 'btn btn-success';
                    toggleBtn.textContent = 'Включить';
                    toggleBtn.setAttribute('onclick', `toggleCheck(${id}, true)`);
                }
            }
        }
        // Инвалидируем кэш проверок для домена
        domainChecksCache.clear();
    } catch (error) {
        showError(`Ошибка: ${error.message}`);
    }
}

// Агрегация результатов по минутам
function aggregateResultsByMinute(results, isHttpCheck = false) {
    if (!results || !Array.isArray(results) || results.length === 0) {
        // Если результатов нет, возвращаем пустые бакеты
        // Исключаем текущую минуту (0), начинаем с -1 минуты
        const now = new Date();
        const buckets = [];
        for (let i = 10; i >= 1; i--) {
            const time = new Date(now.getTime() - i * 60 * 1000);
            time.setSeconds(0, 0);
            const bucket = {
                timestamp: time.toISOString().substring(0, 16).replace('T', ' ') + ':00',
                success_count: 0,
                failure_count: 0,
                avg_latency: 0,
                min_latency: 0,
                max_latency: 0
            };
            if (isHttpCheck) {
                bucket.timeout_count = 0;
                bucket.status_2xx_count = 0;
                bucket.status_4xx_count = 0;
                bucket.status_5xx_count = 0;
            }
            buckets.push(bucket);
        }
        return buckets;
    }
    
    const now = new Date();
    const buckets = {};
    
    // Инициализируем бакеты для последних 10 минут, исключая текущую минуту (0)
    // Начинаем с -1 минуты до -10 минуты
    for (let i = 10; i >= 1; i--) {
        const time = new Date(now.getTime() - i * 60 * 1000);
        // Округляем до минуты
        time.setSeconds(0, 0);
        const key = time.toISOString().substring(0, 16); // YYYY-MM-DDTHH:MM
        buckets[key] = {
            timestamp: time,
            successCount: 0,
            failureCount: 0,
            latencySum: 0,
            latencyCount: 0,
            minLatency: null,
            maxLatency: null
        };
        if (isHttpCheck) {
            buckets[key].timeoutCount = 0;
            buckets[key].status2xxCount = 0;
            buckets[key].status4xxCount = 0;
            buckets[key].status5xxCount = 0;
        }
    }
    
    // Агрегируем результаты
    for (const result of results) {
        if (!result.created_at) continue;
        
        const resultDate = new Date(result.created_at);
        // Округляем до минуты
        resultDate.setSeconds(0, 0);
        const key = resultDate.toISOString().substring(0, 16); // YYYY-MM-DDTHH:MM
        
        if (buckets[key]) {
            if (isHttpCheck && result.outcome) {
                // Для HTTP проверок различаем типы ответов
                if (result.outcome === 'timeout') {
                    buckets[key].timeoutCount++;
                } else if (result.outcome === '2xx') {
                    buckets[key].status2xxCount++;
                    buckets[key].successCount++;
                } else if (result.outcome === '4xx') {
                    buckets[key].status4xxCount++;
                    buckets[key].failureCount++;
                } else if (result.outcome === '5xx') {
                    buckets[key].status5xxCount++;
                    buckets[key].failureCount++;
                } else {
                    // Для других типов ошибок
                    if (result.status === 'success') {
                        buckets[key].successCount++;
                    } else {
                        buckets[key].failureCount++;
                    }
                }
            } else {
                // Для не-HTTP проверок используем стандартную логику
                if (result.status === 'success') {
                    buckets[key].successCount++;
                } else {
                    buckets[key].failureCount++;
                }
            }
            
            if (result.duration_ms) {
                buckets[key].latencySum += result.duration_ms;
                buckets[key].latencyCount++;
                if (buckets[key].minLatency === null || result.duration_ms < buckets[key].minLatency) {
                    buckets[key].minLatency = result.duration_ms;
                }
                if (buckets[key].maxLatency === null || result.duration_ms > buckets[key].maxLatency) {
                    buckets[key].maxLatency = result.duration_ms;
                }
            }
        }
    }
    
    // Преобразуем в массив, сортируем по времени
    return Object.values(buckets)
        .sort((a, b) => a.timestamp - b.timestamp)
        .map(bucket => {
            const result = {
                timestamp: bucket.timestamp.toISOString().substring(0, 16).replace('T', ' ') + ':00',
                success_count: bucket.successCount,
                failure_count: bucket.failureCount,
                avg_latency: bucket.latencyCount > 0 ? bucket.latencySum / bucket.latencyCount : 0,
                min_latency: bucket.minLatency || 0,
                max_latency: bucket.maxLatency || 0
            };
            if (isHttpCheck) {
                result.timeout_count = bucket.timeoutCount || 0;
                result.status_2xx_count = bucket.status2xxCount || 0;
                result.status_4xx_count = bucket.status4xxCount || 0;
                result.status_5xx_count = bucket.status5xxCount || 0;
            }
            return result;
        });
}

// Загрузка графика для проверки в списке (под проверкой)
async function loadCheckChartForCheck(checkId) {
    try {
        // Получаем тип проверки из элемента
        const checkElement = document.getElementById(`check-${checkId}`);
        const checkType = checkElement ? (checkElement.getAttribute('data-check-type') || 'unknown').toLowerCase() : 'unknown';
        const isHttpCheck = checkType === 'http';
        
        // Вычисляем период: последние 10 минут, исключая текущую минуту
        // Запрашиваем данные до 1 минуты назад, чтобы исключить текущую минуту
        const to = new Date();
        to.setSeconds(0, 0); // Округляем до начала текущей минуты
        const from = new Date(to.getTime() - 10 * 60 * 1000); // 10 минут назад
        
        const fromStr = from.toISOString();
        const toStr = to.toISOString();
        
        const url = `/checks/${checkId}/results?from=${encodeURIComponent(fromStr)}&to=${encodeURIComponent(toStr)}&page=1&page_size=100`;
        console.log('Loading chart for check', checkId, 'URL:', url);
        
        // Загружаем результаты проверок
        const response = await apiCall(url);
        
        console.log('Response for check', checkId, ':', response);
        
        if (!response) {
            console.error('No response for check', checkId);
            return;
        }
        
        if (!response.results) {
            console.log('No results in response for check', checkId, ', using empty array');
            response.results = [];
        }
        
        if (!Array.isArray(response.results)) {
            console.error('Results is not an array for check', checkId, 'Response:', response);
            return;
        }

        console.log('Results for check', checkId, ':', response.results.length, 'items');

        const canvasId = `checkChart-${checkId}`;
        console.log('Looking for canvas with ID:', canvasId);
        
        // Небольшая задержка, чтобы убедиться, что DOM обновлен
        await new Promise(resolve => setTimeout(resolve, 100));
        
        const ctx = document.getElementById(canvasId);
        if (!ctx) {
            console.error('Canvas not found for check', checkId, 'Canvas ID:', canvasId);
            // Попробуем найти все canvas элементы для отладки
            const allCanvases = document.querySelectorAll('canvas');
            console.log('All canvases in DOM:', Array.from(allCanvases).map(c => c.id));
            return;
        }
        
        console.log('Canvas found for check', checkId);

        // Агрегируем результаты по минутам (функция обработает пустой массив и вернет 10 пустых бакетов)
        const aggregatedData = aggregateResultsByMinute(response.results, isHttpCheck);
        console.log('Aggregated data for check', checkId, ':', aggregatedData.length, 'buckets');

        // Создаем метки и данные для графика
        const labels = aggregatedData.map(item => {
            const date = new Date(item.timestamp.replace(' ', 'T') + 'Z');
            return date.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' });
        });
        
        // Подготавливаем данные для графика
        let datasets;
        if (isHttpCheck) {
            // Для HTTP проверок показываем разные типы ответов
            const timeoutData = aggregatedData.map(item => item.timeout_count || 0);
            const status2xxData = aggregatedData.map(item => item.status_2xx_count || 0);
            const status4xxData = aggregatedData.map(item => item.status_4xx_count || 0);
            const status5xxData = aggregatedData.map(item => item.status_5xx_count || 0);
            const latencyData = aggregatedData.map(item => item.avg_latency || 0);
            
            datasets = [
                {
                    label: '2xx (Успешные)',
                    data: status2xxData,
                    borderColor: 'rgb(104, 211, 145)',
                    backgroundColor: 'rgba(104, 211, 145, 0.15)',
                    pointBackgroundColor: 'rgb(104, 211, 145)',
                    pointBorderColor: 'rgb(104, 211, 145)',
                    tension: 0.1,
                    yAxisID: 'y'
                },
                {
                    label: '4xx (Ошибки клиента)',
                    data: status4xxData,
                    borderColor: 'rgb(255, 199, 0)',
                    backgroundColor: 'rgba(255, 199, 0, 0.15)',
                    pointBackgroundColor: 'rgb(255, 199, 0)',
                    pointBorderColor: 'rgb(255, 199, 0)',
                    tension: 0.1,
                    yAxisID: 'y'
                },
                {
                    label: '5xx (Ошибки сервера)',
                    data: status5xxData,
                    borderColor: 'rgb(245, 101, 101)',
                    backgroundColor: 'rgba(245, 101, 101, 0.15)',
                    pointBackgroundColor: 'rgb(245, 101, 101)',
                    pointBorderColor: 'rgb(245, 101, 101)',
                    tension: 0.1,
                    yAxisID: 'y'
                },
                {
                    label: 'Таймаут',
                    data: timeoutData,
                    borderColor: 'rgb(108, 117, 125)',
                    backgroundColor: 'rgba(108, 117, 125, 0.1)',
                    pointBackgroundColor: 'rgb(108, 117, 125)',
                    pointBorderColor: 'rgb(108, 117, 125)',
                    tension: 0.1,
                    yAxisID: 'y'
                },
                {
                    label: 'Задержка (мс)',
                    data: latencyData,
                    borderColor: 'rgb(34, 210, 237)',
                    backgroundColor: 'rgba(34, 210, 237, 0.15)',
                    pointBackgroundColor: 'rgb(34, 210, 237)',
                    pointBorderColor: 'rgb(34, 210, 237)',
                    tension: 0.1,
                    yAxisID: 'y1'
                }
            ];
        } else {
            // Для не-HTTP проверок используем стандартное отображение
            const successData = aggregatedData.map(item => item.success_count || 0);
            const failureData = aggregatedData.map(item => item.failure_count || 0);
            const latencyData = aggregatedData.map(item => item.avg_latency || 0);
            
            datasets = [
                {
                    label: 'Успешные',
                    data: successData,
                    borderColor: 'rgb(104, 211, 145)',
                    backgroundColor: 'rgba(104, 211, 145, 0.15)',
                    tension: 0.1,
                    yAxisID: 'y'
                },
                {
                    label: 'Неудачные',
                    data: failureData,
                    borderColor: 'rgb(245, 101, 101)',
                    backgroundColor: 'rgba(245, 101, 101, 0.15)',
                    tension: 0.1,
                    yAxisID: 'y'
                },
                {
                    label: 'Задержка (мс)',
                    data: latencyData,
                    borderColor: 'rgb(34, 210, 237)',
                    backgroundColor: 'rgba(34, 210, 237, 0.15)',
                    tension: 0.1,
                    yAxisID: 'y1'
                }
            ];
        }
        
        console.log('Chart data prepared for check', checkId, ':', { labels: labels.length, datasets: datasets.length, isHttpCheck });

        // Проверяем, существует ли уже график
        let chart;
        if (checkCharts.has(checkId)) {
            // Обновляем существующий график вместо пересоздания
            chart = checkCharts.get(checkId);
            chart.data.labels = labels;
            chart.data.datasets = datasets;
            chart.update('none'); // 'none' - без анимации для плавного обновления
            console.log('Chart updated for check', checkId);
        } else {
            // Создаем новый график только если его еще нет
            console.log('Chart data before creation:', { labels: labels.length, datasets: datasets.length });

            try {
                chart = new Chart(ctx, {
            type: 'line',
            data: {
                labels: labels,
                datasets: datasets
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                interaction: {
                    mode: 'index',
                    intersect: false,
                },
                plugins: {
                    legend: {
                        display: true,
                        position: 'top',
                        labels: {
                            boxWidth: 12,
                            font: {
                                size: 10
                            }
                        }
                    },
                    tooltip: {
                        enabled: true
                    }
                },
                scales: {
                    y: {
                        type: 'linear',
                        display: true,
                        position: 'left',
                        beginAtZero: true,
                        min: 0,
                        title: {
                            display: true,
                            text: 'Количество',
                            font: {
                                size: 10
                            }
                        },
                        ticks: {
                            font: {
                                size: 9
                            },
                            stepSize: 1
                        }
                    },
                    y1: {
                        type: 'linear',
                        display: true,
                        position: 'right',
                        beginAtZero: true,
                        min: 0,
                        title: {
                            display: true,
                            text: 'Задержка (мс)',
                            font: {
                                size: 10
                            }
                        },
                        grid: {
                            drawOnChartArea: false,
                            color: _bsBorderColor,
                        },
                        ticks: {
                            font: {
                                size: 9
                            },
                            stepSize: 10
                        }
                    },
                    x: {
                        ticks: {
                            font: {
                                size: 9
                            }
                        }
                    }
                }
            }
        });
                
                checkCharts.set(checkId, chart);
                console.log('Chart created successfully for check', checkId, 'with', labels.length, 'labels');
            } catch (chartError) {
                console.error('Error creating chart for check', checkId, ':', chartError);
                console.error('Chart error stack:', chartError.stack);
                throw chartError;
            }
        }
        
        // Проверяем, что график действительно создан/обновлен
        console.log('Chart instance:', chart);
        console.log('Chart canvas:', ctx);
        
        if (chart && chart.canvas) {
            console.log('Chart canvas confirmed for check', checkId);
        } else {
            console.error('Chart creation failed - no canvas in chart instance for check', checkId);
        }
    } catch (error) {
        console.error('Error loading check chart for check', checkId, ':', error);
        console.error('Error stack:', error.stack);
        const canvasId = `checkChart-${checkId}`;
        const ctx = document.getElementById(canvasId);
        if (ctx && ctx.parentElement) {
            ctx.parentElement.innerHTML = '<div class="alert alert-danger py-2 mb-0 text-center small">Ошибка загрузки данных: ' + escapeHtml(error.message) + '</div>';
        }
    }
}

// Загрузка графика для проверки в модальном окне
async function loadCheckChart(checkId, interval = '1m') {
    try {
        // Вычисляем период на фронтенде в зависимости от интервала
        const to = new Date();
        let from = new Date();
        
        // Определяем период в зависимости от интервала
        switch (interval) {
            case '1m':
                from = new Date(to.getTime() - 60 * 60 * 1000); // последний час
                break;
            case '5m':
                from = new Date(to.getTime() - 24 * 60 * 60 * 1000); // последние 24 часа
                break;
            case '1h':
                from = new Date(to.getTime() - 7 * 24 * 60 * 60 * 1000); // последняя неделя
                break;
        }
        
        const fromStr = from.toISOString();
        const toStr = to.toISOString();
        
        // Загружаем данные с пагинацией (берем первую страницу, размер 100)
        const response = await apiCall(`/checks/${checkId}/intervals?interval=${interval}&from=${encodeURIComponent(fromStr)}&to=${encodeURIComponent(toStr)}&page=1&page_size=100`);
        
        if (!response || !response.data || !Array.isArray(response.data)) {
            console.error('Invalid chart data format');
            return;
        }

        const ctx = document.getElementById('checkChart');
        if (!ctx) return;

        const labels = response.data.map(item => {
            // Парсим timestamp - может быть в формате "2024-01-01 12:00:00" или ISO
            let date;
            if (item.timestamp.includes('T') || item.timestamp.includes('Z')) {
                date = new Date(item.timestamp);
            } else {
                // Формат "2024-01-01 12:00:00" - добавляем время для парсинга
                date = new Date(item.timestamp.replace(' ', 'T') + 'Z');
            }
            if (isNaN(date.getTime())) {
                console.error('Invalid date:', item.timestamp);
                return item.timestamp;
            }
            if (interval === '1h') {
                return date.toLocaleString('ru-RU', { day: '2-digit', month: '2-digit', hour: '2-digit', minute: '2-digit' });
            }
            return date.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' });
        });
        
        const successData = response.data.map(item => item.success_count || 0);
        const failureData = response.data.map(item => item.failure_count || 0);
        const latencyData = response.data.map(item => item.avg_latency || 0);

        if (checkChart) {
            checkChart.destroy();
        }

        checkChart = new Chart(ctx, {
            type: 'line',
            data: {
                labels: labels,
                datasets: [
                    {
                        label: 'Успешные проверки',
                        data: successData,
                        borderColor: 'rgb(104, 211, 145)',
                        backgroundColor: 'rgba(104, 211, 145, 0.15)',
                        tension: 0.1,
                        yAxisID: 'y'
                    },
                    {
                        label: 'Неудачные проверки',
                        data: failureData,
                        borderColor: 'rgb(245, 101, 101)',
                        backgroundColor: 'rgba(245, 101, 101, 0.15)',
                        tension: 0.1,
                        yAxisID: 'y'
                    },
                    {
                        label: 'Средняя задержка (мс)',
                        data: latencyData,
                        borderColor: 'rgb(34, 210, 237)',
                        backgroundColor: 'rgba(34, 210, 237, 0.15)',
                        tension: 0.1,
                        yAxisID: 'y1'
                    }
                ]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                interaction: {
                    mode: 'index',
                    intersect: false,
                },
                plugins: {
                    legend: {
                        display: true,
                        position: 'top',
                    },
                    tooltip: {
                        enabled: true
                    },
                    zoom: {
                        zoom: {
                            wheel: {
                                enabled: true,
                            },
                            pinch: {
                                enabled: true
                            },
                            mode: 'x',
                        },
                        pan: {
                            enabled: true,
                            mode: 'x',
                        }
                    }
                },
                scales: {
                    y: {
                        type: 'linear',
                        display: true,
                        position: 'left',
                        title: {
                            display: true,
                            text: 'Количество проверок'
                        }
                    },
                    y1: {
                        type: 'linear',
                        display: true,
                        position: 'right',
                        title: {
                            display: true,
                            text: 'Задержка (мс)'
                        },
                        grid: {
                            drawOnChartArea: false,
                            color: _bsBorderColor,
                        },
                    }
                }
            }
        });
    } catch (error) {
        console.error('Error loading check chart:', error);
    }
}

// Просмотр результатов проверки
async function viewCheckResults(checkId) {
    const modalEl = document.getElementById('resultsModal');
    const statsEl = document.getElementById('checkStats');
    const resultsEl = document.getElementById('resultsList');

    currentCheckId = checkId;
    bootstrap.Modal.getOrCreateInstance(modalEl).show();
    statsEl.innerHTML = '<div class="col-12"><p class="text-center text-muted mb-0"><span class="spinner-border spinner-border-sm me-2" role="status"></span>Загрузка статистики...</p></div>';
    resultsEl.innerHTML = '<div class="list-group-item text-center text-muted">Загрузка результатов...</div>';
    
    try {
        // Загружаем статистику
        const stats = await apiCall(`/checks/${checkId}/stats`);
        
        if (!stats || !stats.latency_stats || !stats.status_distribution) {
            statsEl.innerHTML = '<div class="col-12"><div class="alert alert-danger">Ошибка: неверный формат статистики</div></div>';
        } else {
            const total = stats.total_results || 0;
            const avg = stats.latency_stats.avg || 0;
            const p95 = stats.latency_stats.p95 || 0;
            const statusTotal = Object.values(stats.status_distribution || {}).reduce((a, b) => a + b, 0);
            const successCount = stats.status_distribution.success || 0;
            const successRate = statusTotal > 0 ? ((successCount / statusTotal) * 100).toFixed(1) : 0;
            
            statsEl.innerHTML = `
                <div class="col"><div class="card text-center"><div class="card-body py-3"><div class="h4 mb-1">${total}</div><div class="small text-muted">Всего проверок</div></div></div></div>
                <div class="col"><div class="card text-center"><div class="card-body py-3"><div class="h4 mb-1">${avg.toFixed(0)}</div><div class="small text-muted">Средняя задержка (мс)</div></div></div></div>
                <div class="col"><div class="card text-center"><div class="card-body py-3"><div class="h4 mb-1">${p95.toFixed(0)}</div><div class="small text-muted">P95 задержка (мс)</div></div></div></div>
                <div class="col"><div class="card text-center"><div class="card-body py-3"><div class="h4 mb-1">${successRate}%</div><div class="small text-muted">Успешность</div></div></div></div>
            `;
        }
        
        // Загружаем график
        const intervalSelect = document.getElementById('intervalSelect');
        selectedPeriod = intervalSelect.value;
        await loadCheckChart(checkId, intervalSelect.value);
        
        // Обработчик изменения периода
        if (!intervalSelect.hasAttribute('data-handler-added')) {
            intervalSelect.addEventListener('change', async function() {
                if (currentCheckId) {
                    selectedPeriod = this.value;
                    await loadCheckChart(currentCheckId, this.value);
                }
            });
            intervalSelect.setAttribute('data-handler-added', 'true');
        }
        
        // Обработчик кнопки сброса масштаба
        const resetZoomBtn = document.getElementById('resetZoomBtn');
        if (resetZoomBtn && !resetZoomBtn.hasAttribute('data-handler-added')) {
            resetZoomBtn.addEventListener('click', () => {
                if (checkChart) {
                    checkChart.resetZoom();
                }
            });
            resetZoomBtn.setAttribute('data-handler-added', 'true');
        }
        
        // Обработчик кнопки применения периода к графику домена
        const applyPeriodBtn = document.getElementById('applyPeriodBtn');
        if (applyPeriodBtn && !applyPeriodBtn.hasAttribute('data-handler-added')) {
            applyPeriodBtn.addEventListener('click', async () => {
                if (currentDomainId && selectedPeriod) {
                    await applyPeriodToDomainChart(currentDomainId, selectedPeriod);
                    showSuccess('Период применен к графику домена');
                } else {
                    showError('Не выбран период или домен');
                }
            });
            applyPeriodBtn.setAttribute('data-handler-added', 'true');
        }
        
        // Загружаем результаты
        const response = await apiCall(`/checks/${checkId}/results?page=1&page_size=50`);
        
        if (!response || !response.results || !Array.isArray(response.results)) {
            resultsEl.innerHTML = '<div class="list-group-item"><div class="alert alert-danger mb-0">Ошибка: неверный формат результатов</div></div>';
            return;
        }

        if (response.results.length === 0) {
            resultsEl.innerHTML = '<div class="list-group-item text-center text-muted">Нет результатов</div>';
            return;
        }

        resultsEl.innerHTML = '';

        for (const result of response.results) {
            const variant = result.status === 'success' ? 'list-group-item-success' : result.status === 'failure' ? 'list-group-item-danger' : '';
            const statusText = result.status === 'success' ? '✅ Успех' : result.status === 'failure' ? '❌ Ошибка' : '⏱️ Таймаут';
            const resultEl = document.createElement('div');
            resultEl.className = `list-group-item ${variant}`;
            resultEl.innerHTML = `
                <div class="d-flex justify-content-between align-items-center">
                    <span class="fw-bold">${statusText}</span>
                    <span class="small text-muted">${result.created_at ? new Date(result.created_at).toLocaleString('ru-RU') : 'N/A'}</span>
                </div>
                <div class="small text-muted mt-1">
                    Задержка: ${result.duration_ms || 0}мс
                    ${result.status_code ? ` | Код: ${result.status_code}` : ''}
                    ${result.outcome ? ` | ${result.outcome}` : ''}
                    ${result.error_message ? ` | ${escapeHtml(result.error_message)}` : ''}
                </div>
            `;
            resultsEl.appendChild(resultEl);
        }
    } catch (error) {
        statsEl.innerHTML = `<div class="col-12"><div class="alert alert-danger">Ошибка загрузки: ${escapeHtml(error.message)}</div></div>`;
        resultsEl.innerHTML = '';
    }
}

// Bootstrap Modal закрывается через data-bs-dismiss

// Утилита для экранирования HTML
function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

// Графики
let checkChart = null;
let currentCheckId = null;
let currentDomainId = null; // ID домена для текущей проверки
let selectedPeriod = null; // Выбранный период для синхронизации
const domainCharts = new Map(); // Хранилище графиков для каждого домена
const checkCharts = new Map(); // Хранилище графиков для каждой проверки (используется только в модальном окне)

// Создание нового экземпляра графика домена
function createDomainChartInstance(ctx, domainId, labels, successData, failureData, latencyData) {
    const chart = new Chart(ctx, {
        type: 'line',
        data: {
            labels: labels,
            datasets: [
                {
                    label: 'Успешные',
                    data: successData,
                    borderColor: 'rgb(104, 211, 145)',
                    backgroundColor: 'rgba(104, 211, 145, 0.15)',
                    tension: 0.3,
                    borderWidth: 2,
                    pointRadius: 0,
                    pointHitRadius: 8,
                    yAxisID: 'y'
                },
                {
                    label: 'Неудачные',
                    data: failureData,
                    borderColor: 'rgb(245, 101, 101)',
                    backgroundColor: 'rgba(245, 101, 101, 0.15)',
                    tension: 0.3,
                    borderWidth: 2,
                    pointRadius: 0,
                    pointHitRadius: 8,
                    yAxisID: 'y'
                },
                {
                    label: 'Задержка (мс)',
                    data: latencyData,
                    borderColor: 'rgb(34, 210, 237)',
                    backgroundColor: 'rgba(34, 210, 237, 0.15)',
                    tension: 0.3,
                    borderWidth: 2,
                    pointRadius: 0,
                    pointHitRadius: 8,
                    yAxisID: 'y1'
                }
            ]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            animation: false,
            interaction: {
                mode: 'index',
                intersect: false,
            },
            plugins: {
                legend: {
                    display: true,
                    position: 'top',
                    labels: { boxWidth: 12, font: { size: 10 } }
                },
                tooltip: { enabled: true }
            },
            scales: {
                y: {
                    type: 'linear', display: true, position: 'left',
                    beginAtZero: true, min: 0,
                    title: { display: true, text: 'Количество', font: { size: 10 } },
                    ticks: { font: { size: 9 }, stepSize: 1 },
                    grid: { color: _bsBorderColor }
                },
                y1: {
                    type: 'linear', display: true, position: 'right',
                    beginAtZero: true, min: 0,
                    title: { display: true, text: 'Задержка (мс)', font: { size: 10 } },
                    grid: { drawOnChartArea: false, color: _bsBorderColor },
                    ticks: { font: { size: 9 }, stepSize: 10 }
                },
                x: {
                    ticks: { font: { size: 9 } },
                    grid: { color: _bsBorderColor }
                }
            }
        }
    });
    domainCharts.set(domainId, chart);
    return chart;
}

// Загрузка графика для одного домена (используется при первоначальной загрузке и из applyPeriodToDomainChart)
async function loadDomainChart(domainId) {
    const domains = [{ id: domainId }];
    await updateAllDomainCharts(domains);
}

// Просмотр результатов домена (открывает модальное окно с графиком всех проверок)
async function viewDomainResults(domainId) {
    // Загружаем проверки домена и открываем модальное окно с первой проверкой или показываем общий график
    try {
        const checks = await apiCall(`/domains/${domainId}/checks`);
        if (checks && checks.length > 0) {
            // Открываем модальное окно с первой проверкой
            await viewCheckResults(checks[0].id);
        }
    } catch (error) {
        showError(`Ошибка загрузки проверок домена: ${error.message}`);
    }
}

// Применение выбранного периода к графику домена
async function applyPeriodToDomainChart(domainId, period) {
    try {
        // Вычисляем период в зависимости от выбранного интервала
        const to = new Date();
        let from = new Date();
        
        switch (period) {
            case '1m':
                from = new Date(to.getTime() - 60 * 60 * 1000); // последний час
                break;
            case '5m':
                from = new Date(to.getTime() - 24 * 60 * 60 * 1000); // последние 24 часа
                break;
            case '1h':
                from = new Date(to.getTime() - 7 * 24 * 60 * 60 * 1000); // последняя неделя
                break;
            default:
                from = new Date(to.getTime() - 10 * 60 * 1000); // по умолчанию 10 минут
        }
        
        // Обновляем график домена с новым периодом
        const fromStr = from.toISOString();
        const toStr = to.toISOString();
        
        // Загружаем все проверки домена (из кэша или API)
        let checks = domainChecksCache.get(domainId);
        if (!checks) {
            checks = await apiCall(`/domains/${domainId}/checks`);
        }
        if (!checks || !Array.isArray(checks) || checks.length === 0) {
            return;
        }

        // Загружаем результаты всех проверок параллельно
        const resultsArrays = await Promise.all(
            checks.map(check =>
                apiCall(`/checks/${check.id}/results?from=${encodeURIComponent(fromStr)}&to=${encodeURIComponent(toStr)}&page=1&page_size=1000`)
                    .then(r => (r && r.results && Array.isArray(r.results)) ? r.results : [])
                    .catch(() => [])
            )
        );
        const allResults = resultsArrays.flat();
        
        const ctx = document.getElementById(`domainChart-${domainId}`);
        if (!ctx) {
            return;
        }
        
        // Агрегируем результаты по минутам
        const aggregatedData = aggregateResultsByMinute(allResults, false);
        
        // Создаем метки и данные для графика
        const labels = aggregatedData.map(item => {
            const date = new Date(item.timestamp.replace(' ', 'T') + 'Z');
            if (period === '1h') {
                return date.toLocaleString('ru-RU', { day: '2-digit', month: '2-digit', hour: '2-digit', minute: '2-digit' });
            }
            return date.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' });
        });
        
        const successData = aggregatedData.map(item => item.success_count || 0);
        const failureData = aggregatedData.map(item => item.failure_count || 0);
        const latencyData = aggregatedData.map(item => item.avg_latency || 0);
        
        if (domainCharts.has(domainId)) {
            const chart = domainCharts.get(domainId);
            if (chart && chart.canvas && chart.canvas.parentNode) {
                chart.data.labels = labels;
                chart.data.datasets[0].data = successData;
                chart.data.datasets[1].data = failureData;
                chart.data.datasets[2].data = latencyData;
                chart.update('none');
            }
        }
    } catch (error) {
        console.error('Error applying period to domain chart:', error);
        showError(`Ошибка применения периода: ${error.message}`);
    }
}

// Предотвращение параллельных обновлений
let isRefreshing = false;

async function refreshCharts() {
    if (isRefreshing) return;
    isRefreshing = true;
    try {
        const domains = await apiCall('/domains');
        if (domains && Array.isArray(domains)) {
            // Фильтруем только существующие в DOM домены
            const visibleDomains = domains.filter(d => document.getElementById(`domain-${d.id}`));
            if (visibleDomains.length > 0) {
                await updateAllDomainCharts(visibleDomains);
            }
        }
    } catch (error) {
        console.error('Error updating charts:', error);
    } finally {
        isRefreshing = false;
    }
}

// Инициализация при загрузке страницы
document.addEventListener('DOMContentLoaded', () => {
    loadDomains();
    // Обновление графиков каждые 10 секунд
    setInterval(refreshCharts, 10000);
});
