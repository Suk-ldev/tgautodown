// DOM元素
const statusContainer = document.getElementById('status-container');
const loginForm = document.getElementById('login-form');
const codeForm = document.getElementById('code-form');
const successContainer = document.getElementById('success-container');
const errorContainer = document.getElementById('error-container');
const errorText = document.getElementById('error-text');
const downloadList = document.getElementById('download-list');
const downloadEmpty = document.getElementById('download-empty');

// 表单元素
const userForm = document.getElementById('user-form');
const codeSubmitForm = document.getElementById('code-submit-form');

// API基础URL
const API_BASE = window.location.origin;
let isLoggedIn = false;
let currentView = '';
let statusPollTimer = null;
let progressPollTimer = null;

function stopStatusPolling() {
    if (statusPollTimer) {
        clearInterval(statusPollTimer);
        statusPollTimer = null;
    }
}

function stopProgressPolling() {
    if (progressPollTimer) {
        clearInterval(progressPollTimer);
        progressPollTimer = null;
    }
}

function startStatusPolling() {
    stopStatusPolling();
    statusPollTimer = setInterval(() => checkStatus({ showLoading: false }), 2000);
}

function startProgressPolling() {
    stopProgressPolling();
    progressPollTimer = setInterval(loadDownloads, 8000);
}

// 显示/隐藏元素的辅助函数
function showElement(element) {
    element.classList.remove('hidden');
    element.classList.add('fade-in');
}

function hideElement(element) {
    element.classList.add('hidden');
    element.classList.remove('fade-in');
}

// 显示错误信息
function showError(message) {
    stopStatusPolling();
    stopProgressPolling();
    currentView = 'error';
    errorText.textContent = message;
    hideAllContainers();
    showElement(errorContainer);
}

// 隐藏所有容器
function hideAllContainers() {
    [statusContainer, loginForm, codeForm, successContainer, errorContainer].forEach(hideElement);
}

// 检查登录状态
async function checkStatus(options = {}) {
    const showLoading = options.showLoading !== false;
    if (showLoading) {
        hideAllContainers();
        showElement(statusContainer);
        currentView = 'status';
    }

    try {
        const response = await fetch(`${API_BASE}/tgad/login/status`);
        const data = await response.json();

        if (data.rtn === 0) {
            switch (data.status) {
                case 0: // 未登录
                    isLoggedIn = false;
                    showLoginForm();
                    break;
                case 1: // 登录中
                    isLoggedIn = false;
                    showCodeForm();
                    break;
                case 2: // 登录成功
                    isLoggedIn = true;
                    showSuccess(data);
                    break;
                case 3: // 登录失败
                    isLoggedIn = false;
                    showError('登录失败，请重新尝试');
                    break;
                default:
                    isLoggedIn = false;
                    showLoginForm();
            }
        } else {
            showError(`状态检查失败: ${data.msg}`);
        }
    } catch (error) {
        showError(`网络错误: ${error.message}`);
    }
}

// 显示登录表单
function showLoginForm() {
    isLoggedIn = false;
    stopStatusPolling();
    stopProgressPolling();
    if (currentView === 'login') {
        return;
    }
    currentView = 'login';
    hideAllContainers();
    showElement(loginForm);
}

// 显示验证码表单
function showCodeForm() {
    isLoggedIn = false;
    stopProgressPolling();
    if (currentView === 'code') {
        return;
    }
    currentView = 'code';
    hideAllContainers();
    showElement(codeForm);
}

// 显示成功消息
function showSuccess() {
    isLoggedIn = true;
    stopStatusPolling();
    if (currentView === 'success') {
        loadDownloads();
        return;
    }
    currentView = 'success';
    hideAllContainers();
    showElement(successContainer);
    loadDownloads();
    startProgressPolling();
}

function formatBytes(size) {
    if (!size) {
        return '0 B';
    }
    if (size >= 1073741824) {
        return `${(size / 1073741824).toFixed(2)} GB`;
    }
    if (size >= 1048576) {
        return `${(size / 1048576).toFixed(2)} MB`;
    }
    if (size >= 1024) {
        return `${(size / 1024).toFixed(2)} KB`;
    }
    return `${size} B`;
}

function stateText(state) {
    const states = {
        queued: '排队中',
        downloading: '下载中',
        paused: '已暂停'
    };
    return states[state] || state || '-';
}

function escapeHtml(value) {
    return String(value || '').replace(/[&<>"']/g, (char) => ({
        '&': '&amp;',
        '<': '&lt;',
        '>': '&gt;',
        '"': '&quot;',
        "'": '&#39;'
    })[char]);
}

async function loadDownloads() {
    if (!downloadList || !downloadEmpty) {
        return;
    }

    try {
        const response = await fetch(`${API_BASE}/tgad/downloads/progress`);
        const data = await response.json();

        if (data.rtn !== 0) {
            downloadList.innerHTML = `<div class="download-empty">进度获取失败: ${data.msg}</div>`;
            downloadEmpty.classList.add('hidden');
            return;
        }

        const downloads = data.downloads || [];
        if (downloads.length === 0) {
            downloadList.innerHTML = '';
            downloadEmpty.classList.remove('hidden');
            return;
        }

        downloadEmpty.classList.add('hidden');
        downloadList.innerHTML = downloads.map((item) => {
            const percent = Math.max(0, Math.min(100, item.percent || 0));
            const filename = escapeHtml(item.filename || '-');
            return `
                <div class="download-item">
                    <div class="download-meta">
                        <span class="download-uid">UID ${item.uid}</span>
                        <span class="download-state">${stateText(item.state)}</span>
                    </div>
                    <div class="download-name" title="${filename}">${filename}</div>
                    <div class="progress-bar">
                        <div class="progress-fill" style="width: ${percent}%"></div>
                    </div>
                    <div class="download-size">
                        <span>${formatBytes(item.downloaded)} / ${formatBytes(item.total)}</span>
                        <span>${percent}%</span>
                    </div>
                </div>
            `;
        }).join('');
    } catch (error) {
        downloadList.innerHTML = `<div class="download-empty">网络错误: ${error.message}</div>`;
        downloadEmpty.classList.add('hidden');
    }
}

// 提交用户登录信息
async function submitUserInfo(userData) {
    hideAllContainers();
    showElement(statusContainer);

    try {
        const response = await fetch(`${API_BASE}/tgad/login/user`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify(userData),
        });

        const data = await response.json();

        if (data.rtn === 0) {
            showCodeForm();
        } else {
            showError(`提交失败: ${data.msg}`);
        }
    } catch (error) {
        showError(`网络错误: ${error.message}`);
    }
}

// 提交验证码
async function submitCode(code) {
    hideAllContainers();
    showElement(statusContainer);

    try {
        const response = await fetch(`${API_BASE}/tgad/login/code`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({ code }),
        });

        const data = await response.json();

        if (data.rtn === 0) {
            // 验证码提交成功，等待登录完成
            statusContainer.innerHTML = '<div class="loading">正在验证登录状态...</div>';
            currentView = 'status';
            setTimeout(() => checkStatus({ showLoading: false }), 1500);
            startStatusPolling();
        } else {
            showError(`验证码提交失败: ${data.msg}`);
        }
    } catch (error) {
        showError(`网络错误: ${error.message}`);
    }
}

// 事件监听器
userForm.addEventListener('submit', async (e) => {
    e.preventDefault();
    
    const formData = new FormData(userForm);
    const userData = {
        appid: parseInt(formData.get('appid')),
        apphash: formData.get('apphash'),
        phone: formData.get('phone')
    };

    await submitUserInfo(userData);
});

codeSubmitForm.addEventListener('submit', async (e) => {
    e.preventDefault();
    
    const formData = new FormData(codeSubmitForm);
    const code = formData.get('code');
    
    await submitCode(code);
});

// 页面加载时检查状态
document.addEventListener('DOMContentLoaded', () => {
    checkStatus();
});

// 全局函数，用于重试按钮
window.checkStatus = checkStatus;
window.loadDownloads = loadDownloads;
