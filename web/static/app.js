const API_BASE = '';

// Утилиты
function showError(message) {
    const errorDiv = document.createElement('div');
    errorDiv.className = 'error';
    errorDiv.textContent = message;
    document.querySelector('.main-content').insertBefore(errorDiv, document.querySelector('.main-content').firstChild);
    setTimeout(() => errorDiv.remove(), 5000);
}

function showSuccess(message) {
    const successDiv = document.createElement('div');
    successDiv.className = 'success';
    successDiv.textContent = message;
    document.querySelector('.main-content').insertBefore(successDiv, document.querySelector('.main-content').firstChild);
    setTimeout(() => successDiv.remove(), 3000);
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

// Загрузка доменов
async function loadDomains() {
    const listEl = document.getElementById('domainsList');
    listEl.innerHTML = '<p class="loading">Загрузка...</p>';
    
    // Очищаем все графики проверок
    checkCharts.forEach((chart, checkId) => {
        chart.destroy();
    });
    checkCharts.clear();
    
    try {
        const domains = await apiCall('/domains');
        
        if (!domains || !Array.isArray(domains)) {
            listEl.innerHTML = '<div class="error">Ошибка: неверный формат ответа от сервера</div>';
            return;
        }
        
        if (domains.length === 0) {
            listEl.innerHTML = '<div class="empty-state">Нет доменов для мониторинга</div>';
            return;
        }
        
        listEl.innerHTML = '';
        
        for (const domain of domains) {
            const domainEl = createDomainElement(domain);
            listEl.appendChild(domainEl);
            await loadChecksForDomain(domain.id);
        }
    } catch (error) {
        listEl.innerHTML = `<div class="error">Ошибка загрузки: ${error.message}</div>`;
    }
}

function createDomainElement(domain) {
    const div = document.createElement('div');
    div.className = 'domain-item';
    div.id = `domain-${domain.id}`;
    div.innerHTML = `
        <div class="domain-header">
            <span class="domain-name">${escapeHtml(domain.name)}</span>
            <div class="domain-actions">
                <button class="btn btn-primary btn-small" onclick="openCheckModal(${domain.id})">+ Проверка</button>
                <button class="btn btn-danger btn-small" onclick="deleteDomain(${domain.id})">Удалить</button>
            </div>
        </div>
        <div class="checks-list" id="checks-${domain.id}">
            <p class="loading">Загрузка проверок...</p>
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
            checksEl.innerHTML = '<div class="error">Ошибка: неверный формат ответа от сервера</div>';
            return;
        }
        
        if (checks.length === 0) {
            checksEl.innerHTML = '<p style="color: #999; text-align: center; padding: 10px;">Нет проверок</p>';
            return;
        }
        
        checksEl.innerHTML = '';
        
        for (const check of checks) {
            const checkEl = createCheckElement(check);
            checksEl.appendChild(checkEl);
            // Небольшая задержка, чтобы canvas успел отрендериться в DOM
            await new Promise(resolve => setTimeout(resolve, 50));
            // Загружаем график для проверки
            await loadCheckChartForCheck(check.id);
        }
    } catch (error) {
        checksEl.innerHTML = `<div class="error">Ошибка загрузки проверок: ${error.message}</div>`;
    }
}

function createCheckElement(check) {
    const div = document.createElement('div');
    div.className = 'check-item';
    div.id = `check-${check.id}`;
    
    const statusClass = check.enabled ? 'enabled' : 'disabled';
    const statusText = check.enabled ? 'Включена' : 'Отключена';
    
    let details = `Интервал: ${check.interval_seconds || 0}с`;
    if (check.params) {
        if (check.params.path) details += ` | Путь: ${check.params.path}`;
        if (check.params.port) details += ` | Порт: ${check.params.port}`;
    }
    if (check.realtime_mode) details += ` | Реальное время`;
    
    const checkType = (check.type || 'unknown').toLowerCase();
    
    div.innerHTML = `
        <div style="display: flex; justify-content: space-between; align-items: center;">
            <div class="check-info">
                <span class="check-type ${checkType}">${checkType.toUpperCase()}</span>
                <span class="check-status ${statusClass}">${statusText}</span>
                <div class="check-details">${details}</div>
            </div>
            <div class="check-actions">
                <button class="btn btn-primary btn-small" onclick="viewCheckResults(${check.id})">Результаты</button>
                ${check.enabled 
                    ? `<button class="btn btn-small" onclick="toggleCheck(${check.id}, false)">Отключить</button>`
                    : `<button class="btn btn-success btn-small" onclick="toggleCheck(${check.id}, true)">Включить</button>`
                }
                <button class="btn btn-danger btn-small" onclick="deleteCheck(${check.id})">Удалить</button>
            </div>
        </div>
        <div class="check-chart-container" style="margin-top: 15px; width: 100%; position: relative; height: 150px;">
            <canvas id="checkChart-${check.id}"></canvas>
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
    document.getElementById('checkModal').style.display = 'block';
    
    // Показываем/скрываем поля в зависимости от типа проверки
    if (!checkTypeHandlerAdded) {
        document.getElementById('checkType').addEventListener('change', updateCheckForm);
        checkTypeHandlerAdded = true;
    }
    updateCheckForm();
}

function updateCheckForm() {
    const type = document.getElementById('checkType').value;
    const httpParams = document.getElementById('httpParams');
    const portParams = document.getElementById('portParams');
    const udpParams = document.getElementById('udpParams');
    
    httpParams.style.display = type === 'http' ? 'block' : 'none';
    portParams.style.display = (type === 'tcp' || type === 'udp') ? 'block' : 'none';
    udpParams.style.display = type === 'udp' ? 'block' : 'none';
}

document.getElementById('addCheckForm').addEventListener('submit', async (e) => {
    e.preventDefault();
    
    const domainId = parseInt(document.getElementById('checkDomainId').value);
    const type = document.getElementById('checkType').value;
    const interval = parseInt(document.getElementById('checkInterval').value);
    const timeout = parseInt(document.getElementById('checkTimeout').value) || 5000;
    const realtime = document.getElementById('checkRealtime').checked;
    const rateLimit = parseInt(document.getElementById('checkRateLimit').value) || 0;
    
    const params = {};
    if (type === 'http') {
        params.path = document.getElementById('checkPath').value || '/';
    }
    if (type === 'tcp' || type === 'udp') {
        const port = parseInt(document.getElementById('checkPort').value);
        if (!port || port < 1 || port > 65535) {
            showError('Укажите корректный порт (1-65535)');
            return;
        }
        params.port = port;
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
                interval_seconds: interval,
                params,
                realtime_mode: realtime,
                rate_limit_per_minute: rateLimit
            })
        });
        
        document.getElementById('checkModal').style.display = 'none';
        document.getElementById('addCheckForm').reset();
        showSuccess('Проверка создана');
        await loadChecksForDomain(domainId);
    } catch (error) {
        showError(`Ошибка создания проверки: ${error.message}`);
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
        
        const checkEl = document.getElementById(`check-${id}`);
        if (checkEl) {
            checkEl.remove();
        } else {
            // Если элемент не найден, перезагружаем домены
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
        
        // Перезагружаем все домены для обновления статусов
        await loadDomains();
    } catch (error) {
        showError(`Ошибка: ${error.message}`);
    }
}

// Агрегация результатов по минутам
function aggregateResultsByMinute(results) {
    if (!results || !Array.isArray(results) || results.length === 0) {
        // Если результатов нет, возвращаем пустые бакеты
        const now = new Date();
        const buckets = [];
        for (let i = 9; i >= 0; i--) {
            const time = new Date(now.getTime() - i * 60 * 1000);
            buckets.push({
                timestamp: time.toISOString().substring(0, 16).replace('T', ' ') + ':00',
                success_count: 0,
                failure_count: 0,
                avg_latency: 0,
                min_latency: 0,
                max_latency: 0
            });
        }
        return buckets;
    }
    
    const now = new Date();
    const buckets = {};
    
    // Инициализируем бакеты для последних 10 минут
    for (let i = 9; i >= 0; i--) {
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
    }
    
    // Агрегируем результаты
    for (const result of results) {
        if (!result.created_at) continue;
        
        const resultDate = new Date(result.created_at);
        // Округляем до минуты
        resultDate.setSeconds(0, 0);
        const key = resultDate.toISOString().substring(0, 16); // YYYY-MM-DDTHH:MM
        
        if (buckets[key]) {
            if (result.status === 'success') {
                buckets[key].successCount++;
            } else {
                buckets[key].failureCount++;
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
        .map(bucket => ({
            timestamp: bucket.timestamp.toISOString().substring(0, 16).replace('T', ' ') + ':00',
            success_count: bucket.successCount,
            failure_count: bucket.failureCount,
            avg_latency: bucket.latencyCount > 0 ? bucket.latencySum / bucket.latencyCount : 0,
            min_latency: bucket.minLatency || 0,
            max_latency: bucket.maxLatency || 0
        }));
}

// Загрузка графика для проверки в списке (под проверкой)
async function loadCheckChartForCheck(checkId) {
    try {
        // Вычисляем период: последние 10 минут
        const to = new Date();
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

        // Удаляем старый график, если есть
        if (checkCharts.has(checkId)) {
            checkCharts.get(checkId).destroy();
            checkCharts.delete(checkId);
        }

        // Агрегируем результаты по минутам (функция обработает пустой массив и вернет 10 пустых бакетов)
        const aggregatedData = aggregateResultsByMinute(response.results);
        console.log('Aggregated data for check', checkId, ':', aggregatedData.length, 'buckets');

        // Создаем метки и данные для графика
        const labels = aggregatedData.map(item => {
            const date = new Date(item.timestamp.replace(' ', 'T') + 'Z');
            return date.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' });
        });
        
        const successData = aggregatedData.map(item => item.success_count || 0);
        const failureData = aggregatedData.map(item => item.failure_count || 0);
        const latencyData = aggregatedData.map(item => item.avg_latency || 0);
        
        console.log('Chart data prepared for check', checkId, ':', { labels: labels.length, successData, failureData, latencyData });

        console.log('Chart data before creation:', { labels: labels.length, successData: successData.length, failureData: failureData.length, latencyData: latencyData.length });
        console.log('Sample labels:', labels.slice(0, 3));
        console.log('Sample successData:', successData.slice(0, 3));

        let chart;
        try {
            chart = new Chart(ctx, {
            type: 'line',
            data: {
                labels: labels,
                datasets: [
                    {
                        label: 'Успешные',
                        data: successData,
                        borderColor: 'rgb(39, 174, 96)',
                        backgroundColor: 'rgba(39, 174, 96, 0.1)',
                        tension: 0.1,
                        yAxisID: 'y'
                    },
                    {
                        label: 'Неудачные',
                        data: failureData,
                        borderColor: 'rgb(231, 76, 60)',
                        backgroundColor: 'rgba(231, 76, 60, 0.1)',
                        tension: 0.1,
                        yAxisID: 'y'
                    },
                    {
                        label: 'Задержка (мс)',
                        data: latencyData,
                        borderColor: 'rgb(102, 126, 234)',
                        backgroundColor: 'rgba(102, 126, 234, 0.1)',
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
        console.log('Chart instance:', chart);
        console.log('Chart canvas:', ctx);
        
        // Проверяем, что график действительно создан
        if (chart && chart.canvas) {
            console.log('Chart canvas confirmed for check', checkId);
        } else {
            console.error('Chart creation failed - no canvas in chart instance for check', checkId);
        }
    } catch (chartError) {
        console.error('Error creating chart for check', checkId, ':', chartError);
        console.error('Chart error stack:', chartError.stack);
        throw chartError;
    }
    } catch (error) {
        console.error('Error loading check chart for check', checkId, ':', error);
        console.error('Error stack:', error.stack);
        const canvasId = `checkChart-${checkId}`;
        const ctx = document.getElementById(canvasId);
        if (ctx && ctx.parentElement) {
            ctx.parentElement.innerHTML = '<p style="color: #e74c3c; text-align: center; padding: 10px; font-size: 0.9em;">Ошибка загрузки данных: ' + error.message + '</p>';
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
                        borderColor: 'rgb(39, 174, 96)',
                        backgroundColor: 'rgba(39, 174, 96, 0.1)',
                        tension: 0.1,
                        yAxisID: 'y'
                    },
                    {
                        label: 'Неудачные проверки',
                        data: failureData,
                        borderColor: 'rgb(231, 76, 60)',
                        backgroundColor: 'rgba(231, 76, 60, 0.1)',
                        tension: 0.1,
                        yAxisID: 'y'
                    },
                    {
                        label: 'Средняя задержка (мс)',
                        data: latencyData,
                        borderColor: 'rgb(102, 126, 234)',
                        backgroundColor: 'rgba(102, 126, 234, 0.1)',
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
    const modal = document.getElementById('resultsModal');
    const statsEl = document.getElementById('checkStats');
    const resultsEl = document.getElementById('resultsList');
    
    currentCheckId = checkId;
    modal.style.display = 'block';
    statsEl.innerHTML = '<p class="loading">Загрузка статистики...</p>';
    resultsEl.innerHTML = '<p class="loading">Загрузка результатов...</p>';
    
    try {
        // Загружаем статистику
        const stats = await apiCall(`/checks/${checkId}/stats`);
        
        if (!stats || !stats.latency_stats || !stats.status_distribution) {
            statsEl.innerHTML = '<div class="error">Ошибка: неверный формат статистики</div>';
        } else {
            const total = stats.total_results || 0;
            const avg = stats.latency_stats.avg || 0;
            const p95 = stats.latency_stats.p95 || 0;
            const statusTotal = Object.values(stats.status_distribution || {}).reduce((a, b) => a + b, 0);
            const successCount = stats.status_distribution.success || 0;
            const successRate = statusTotal > 0 ? ((successCount / statusTotal) * 100).toFixed(1) : 0;
            
            statsEl.innerHTML = `
                <div class="stat-card">
                    <div class="stat-value">${total}</div>
                    <div class="stat-label">Всего проверок</div>
                </div>
                <div class="stat-card">
                    <div class="stat-value">${avg.toFixed(0)}</div>
                    <div class="stat-label">Средняя задержка (мс)</div>
                </div>
                <div class="stat-card">
                    <div class="stat-value">${p95.toFixed(0)}</div>
                    <div class="stat-label">P95 задержка (мс)</div>
                </div>
                <div class="stat-card">
                    <div class="stat-value">${successRate}%</div>
                    <div class="stat-label">Успешность</div>
                </div>
            `;
        }
        
        // Загружаем график
        const intervalSelect = document.getElementById('intervalSelect');
        await loadCheckChart(checkId, intervalSelect.value);
        
        // Обработчик изменения периода
        if (!intervalSelect.onchange) {
            intervalSelect.onchange = async function() {
                if (currentCheckId) {
                    await loadCheckChart(currentCheckId, this.value);
                }
            };
        }
        
        // Загружаем результаты
        const response = await apiCall(`/checks/${checkId}/results?page=1&page_size=50`);
        
        if (!response || !response.results || !Array.isArray(response.results)) {
            resultsEl.innerHTML = '<div class="error">Ошибка: неверный формат результатов</div>';
            return;
        }
        
        if (response.results.length === 0) {
            resultsEl.innerHTML = '<div class="empty-state">Нет результатов</div>';
            return;
        }
        
        resultsEl.innerHTML = '';
        
        for (const result of response.results) {
            const resultEl = document.createElement('div');
            resultEl.className = `result-item ${result.status || 'unknown'}`;
            
            const statusText = result.status === 'success' ? '✅ Успех' : 
                             result.status === 'failure' ? '❌ Ошибка' : '⏱️ Таймаут';
            
            resultEl.innerHTML = `
                <div class="result-header">
                    <span class="result-status">${statusText}</span>
                    <span class="result-time">${result.created_at ? new Date(result.created_at).toLocaleString('ru-RU') : 'N/A'}</span>
                </div>
                <div class="result-details">
                    Задержка: ${result.duration_ms || 0}мс
                    ${result.status_code ? ` | Код: ${result.status_code}` : ''}
                    ${result.outcome ? ` | ${result.outcome}` : ''}
                    ${result.error_message ? ` | ${escapeHtml(result.error_message)}` : ''}
                </div>
            `;
            
            resultsEl.appendChild(resultEl);
        }
    } catch (error) {
        statsEl.innerHTML = `<div class="error">Ошибка загрузки: ${error.message}</div>`;
        resultsEl.innerHTML = '';
    }
}

// Закрытие модальных окон
document.querySelectorAll('.close').forEach(closeBtn => {
    closeBtn.addEventListener('click', function() {
        this.closest('.modal').style.display = 'none';
    });
});

window.addEventListener('click', function(e) {
    if (e.target.classList.contains('modal')) {
        e.target.style.display = 'none';
    }
});

// Утилита для экранирования HTML
function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

// Графики
let dashboardChart = null;
let checkChart = null;
let currentCheckId = null;
const checkCharts = new Map(); // Хранилище графиков для каждой проверки

async function loadDashboardChart() {
    try {
        // Вычисляем период на фронтенде: последние 10 минут
        const to = new Date();
        const from = new Date(to.getTime() - 10 * 60 * 1000); // 10 минут назад
        
        const fromStr = from.toISOString();
        const toStr = to.toISOString();
        
        // Загружаем все результаты за последние 10 минут
        const url = `/results`;
        console.log('Loading dashboard chart, URL:', url);
        
        // Загружаем все результаты
        const allResults = await apiCall(url);
        
        console.log('Dashboard chart response:', allResults);
        
        if (!allResults || !Array.isArray(allResults)) {
            console.error('Invalid response for dashboard chart');
            return;
        }

        // Фильтруем результаты по времени
        const filteredResults = allResults.filter(result => {
            if (!result.created_at) return false;
            const resultDate = new Date(result.created_at);
            return resultDate >= from && resultDate <= to;
        });

        console.log('Filtered results for dashboard:', filteredResults.length);

        const ctx = document.getElementById('dashboardChart');
        if (!ctx) {
            console.error('Dashboard chart canvas not found');
            return;
        }

        // Агрегируем результаты по минутам
        const aggregatedData = aggregateResultsByMinute(filteredResults);
        console.log('Aggregated dashboard data:', aggregatedData);

        // Создаем метки и данные для графика
        const labels = aggregatedData.map(item => {
            const date = new Date(item.timestamp.replace(' ', 'T') + 'Z');
            return date.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' });
        });
        
        const successData = aggregatedData.map(item => item.success_count || 0);
        const failureData = aggregatedData.map(item => item.failure_count || 0);
        const latencyData = aggregatedData.map(item => item.avg_latency || 0);
        
        console.log('Dashboard chart data prepared:', { labels: labels.length, successData, failureData, latencyData });

        if (dashboardChart) {
            dashboardChart.destroy();
        }

        console.log('Creating dashboard chart with', labels.length, 'labels');

        dashboardChart = new Chart(ctx, {
            type: 'line',
            data: {
                labels: labels,
                datasets: [
                    {
                        label: 'Успешные проверки',
                        data: successData,
                        borderColor: 'rgb(39, 174, 96)',
                        backgroundColor: 'rgba(39, 174, 96, 0.1)',
                        tension: 0.1,
                        yAxisID: 'y'
                    },
                    {
                        label: 'Неудачные проверки',
                        data: failureData,
                        borderColor: 'rgb(231, 76, 60)',
                        backgroundColor: 'rgba(231, 76, 60, 0.1)',
                        tension: 0.1,
                        yAxisID: 'y'
                    },
                    {
                        label: 'Средняя задержка (мс)',
                        data: latencyData,
                        borderColor: 'rgb(102, 126, 234)',
                        backgroundColor: 'rgba(102, 126, 234, 0.1)',
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
                            text: 'Количество проверок'
                        },
                        ticks: {
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
                            text: 'Задержка (мс)'
                        },
                        grid: {
                            drawOnChartArea: false,
                        },
                        ticks: {
                            stepSize: 10
                        }
                    }
                }
            }
        });
        
        console.log('Dashboard chart created successfully');
    } catch (error) {
        console.error('Error loading dashboard chart:', error);
    }
}

// Инициализация при загрузке страницы
document.addEventListener('DOMContentLoaded', () => {
    loadDomains();
    loadDashboardChart();
    // Автообновление каждые 30 секунд
    setInterval(loadDomains, 30000);
    // Автообновление графика каждые 30 секунд
    setInterval(loadDashboardChart, 30000);
});
