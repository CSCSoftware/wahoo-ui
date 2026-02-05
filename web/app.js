// State
let currentChatJID = null;
let chats = [];

// DOM Elements
const chatList = document.getElementById('chat-list');
const chatView = document.getElementById('chat-view');
const noChat = document.getElementById('no-chat');
const chatName = document.getElementById('chat-name');
const chatJidEl = document.getElementById('chat-jid');
const messagesContainer = document.getElementById('messages');
const messageInput = document.getElementById('message-text');
const sendBtn = document.getElementById('send-btn');
const searchInput = document.getElementById('search');
const statusEl = document.getElementById('status');

// Initialize
document.addEventListener('DOMContentLoaded', () => {
    loadChats();
    checkStatus();
    setInterval(checkStatus, 5000);

    sendBtn.addEventListener('click', sendMessage);
    messageInput.addEventListener('keypress', (e) => {
        if (e.key === 'Enter') sendMessage();
    });

    searchInput.addEventListener('input', filterChats);
});

// API calls
async function loadChats() {
    try {
        const res = await fetch('/api/chats?limit=100');
        chats = await res.json();
        renderChats(chats);
    } catch (err) {
        console.error('Failed to load chats:', err);
    }
}

async function loadMessages(chatJID) {
    try {
        const res = await fetch(`/api/messages?chat_jid=${encodeURIComponent(chatJID)}&limit=100`);
        const messages = await res.json();
        renderMessages(messages);
    } catch (err) {
        console.error('Failed to load messages:', err);
    }
}

async function sendMessage() {
    const text = messageInput.value.trim();
    if (!text || !currentChatJID) return;

    sendBtn.disabled = true;
    try {
        const res = await fetch('/api/send', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                recipient: currentChatJID,
                message: text
            })
        });
        const result = await res.json();
        if (result.success) {
            messageInput.value = '';
            // Reload messages after short delay
            setTimeout(() => loadMessages(currentChatJID), 500);
        } else {
            alert('Fehler: ' + result.message);
        }
    } catch (err) {
        alert('Senden fehlgeschlagen: ' + err.message);
    } finally {
        sendBtn.disabled = false;
    }
}

async function checkStatus() {
    try {
        const res = await fetch('/api/status');
        const status = await res.json();
        if (status.connected) {
            statusEl.textContent = 'Verbunden';
            statusEl.className = 'status connected';
        } else {
            statusEl.textContent = 'Getrennt';
            statusEl.className = 'status disconnected';
        }
    } catch (err) {
        statusEl.textContent = 'Fehler';
        statusEl.className = 'status disconnected';
    }
}

// Rendering
function renderChats(chatArray) {
    chatList.innerHTML = chatArray.map(chat => {
        const name = chat.name || chat.jid;
        const initial = name.charAt(0).toUpperCase();
        const isGroup = chat.is_group;
        const preview = chat.last_message || '';
        const time = formatTime(chat.last_message_time);
        const sender = chat.last_sender ? (chat.last_is_from_me ? '' : chat.last_sender + ': ') : '';

        return `
            <div class="chat-item ${chat.jid === currentChatJID ? 'active' : ''}"
                 data-jid="${escapeHtml(chat.jid)}">
                <div class="chat-avatar ${isGroup ? 'group' : ''}">${initial}</div>
                <div class="chat-content">
                    <div class="chat-name">${escapeHtml(name)}</div>
                    <div class="chat-preview">${escapeHtml(sender + preview)}</div>
                </div>
                <div class="chat-meta">
                    <span class="chat-time">${time}</span>
                </div>
            </div>
        `;
    }).join('');

    // Add click handlers
    chatList.querySelectorAll('.chat-item').forEach(item => {
        item.addEventListener('click', () => selectChat(item.dataset.jid));
    });
}

function renderMessages(messages) {
    // Reverse to show oldest first
    const sorted = [...messages].reverse();

    messagesContainer.innerHTML = sorted.map(msg => {
        const isOutgoing = msg.is_from_me;
        const time = formatTime(msg.timestamp);
        const hasMedia = msg.media_type && msg.media_type !== '';

        return `
            <div class="message ${isOutgoing ? 'outgoing' : 'incoming'}">
                ${!isOutgoing ? `<div class="message-sender">${escapeHtml(msg.sender)}</div>` : ''}
                ${hasMedia ? `<div class="message-media">[${msg.media_type}]</div>` : ''}
                <div class="message-text">${escapeHtml(msg.content || '')}</div>
                <div class="message-time">${time}</div>
            </div>
        `;
    }).join('');

    // Scroll to bottom
    messagesContainer.scrollTop = messagesContainer.scrollHeight;
}

function selectChat(jid) {
    currentChatJID = jid;

    // Update UI
    noChat.classList.add('hidden');
    chatView.classList.remove('hidden');

    // Find chat info
    const chat = chats.find(c => c.jid === jid);
    if (chat) {
        chatName.textContent = chat.name || chat.jid;
        chatJidEl.textContent = jid;
    }

    // Mark as active
    chatList.querySelectorAll('.chat-item').forEach(item => {
        item.classList.toggle('active', item.dataset.jid === jid);
    });

    // Load messages
    loadMessages(jid);
}

function filterChats() {
    const query = searchInput.value.toLowerCase();
    const filtered = chats.filter(chat => {
        const name = (chat.name || chat.jid).toLowerCase();
        return name.includes(query);
    });
    renderChats(filtered);
}

// Helpers
function formatTime(timestamp) {
    if (!timestamp) return '';
    const date = new Date(timestamp);
    const now = new Date();
    const isToday = date.toDateString() === now.toDateString();

    if (isToday) {
        return date.toLocaleTimeString('de-DE', { hour: '2-digit', minute: '2-digit' });
    } else {
        return date.toLocaleDateString('de-DE', { day: '2-digit', month: '2-digit' });
    }
}

function escapeHtml(text) {
    if (!text) return '';
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}
