/**
 * CHATGPT BROWSER BRIDGE (Automation Robot)
 * 
 * This script connects to your existing, logged-in Chrome browser.
 * It will automatically:
 * 1. Read trade signals from the Go engine.
 * 2. Type them into your ChatGPT tab.
 * 3. Read the AI's JSON response.
 * 4. Send the structured verdict back to the engine.
 * 
 * TO RUN:
 * 1. Close all Chrome windows.
 * 2. Start Chrome with: "chrome.exe --remote-debugging-port=9222"
 * 3. Log into chatgpt.com
 * 4. Run: node bridge.js
 */

const puppeteer = require('puppeteer-core');
const axios = require('axios');
const fs = require('fs');
const path = require('path');

const ENGINE_URL = process.env.ENGINE_URL || (() => {
    console.warn('⚠️  ENGINE_URL not set — falling back to Render cloud. Set ENGINE_URL=http://localhost:8080 for local engine.');
    return 'https://antigravity-x7he.onrender.com';
})();
const POLL_INTERVAL = 3000;
const RESPONSE_TIMEOUT_MS = 90000;
const RESPONSE_STABLE_MS = 2500;
const RESPONSE_POLL_MS = 800;
const BRIDGE_LOG_PATH = path.join(__dirname, 'bridge-decisions.jsonl');
const inFlightSignals = new Set();

function appendBridgeLog(entry) {
    const line = JSON.stringify({
        ts: new Date().toISOString(),
        ...entry,
    }) + '\n';
    fs.appendFileSync(BRIDGE_LOG_PATH, line, 'utf8');
}

async function postBridgeEvent(message, level = 'info') {
    try {
        await axios.post(`${ENGINE_URL}/api/ai/bridge-event`, {
            message,
            level,
        });
    } catch (err) {
        appendBridgeLog({
            stage: 'bridge_event_post_error',
            message,
            level,
            error: err.message,
        });
    }
}

function extractJson(text) {
    const trimmed = String(text || '').trim();
    const fenceMatch = trimmed.match(/```(?:json)?\s*([\s\S]*?)```/i);
    const candidate = fenceMatch ? fenceMatch[1].trim() : trimmed;
    const start = candidate.indexOf('{');
    const end = candidate.lastIndexOf('}');
    if (start === -1 || end === -1 || end <= start) {
        throw new Error('No JSON object found in ChatGPT response');
    }
    return JSON.parse(candidate.slice(start, end + 1));
}

async function readLastAssistantMessage(chatPage) {
    return chatPage.evaluate(() => {
        const selectors = [
            '[data-message-author-role="assistant"]',
            '.markdown.prose',
        ];

        for (const selector of selectors) {
            const nodes = Array.from(document.querySelectorAll(selector));
            if (nodes.length > 0) {
                const last = nodes[nodes.length - 1];
                return last.innerText || last.textContent || '';
            }
        }
        return '';
    });
}

async function getComposerState(chatPage) {
    return chatPage.evaluate(() => {
        const selectors = [
            '#prompt-textarea',
            'div[contenteditable="true"][id="prompt-textarea"]',
            'textarea[placeholder]',
            'div[contenteditable="true"][data-testid*="composer"]',
            'div[contenteditable="true"][data-testid*="prompt"]',
        ];

        let composer = null;
        for (const selector of selectors) {
            const candidate = document.querySelector(selector);
            if (candidate) {
                composer = candidate;
                break;
            }
        }

        const sendSelectors = [
            'button[data-testid="send-button"]',
            'button[aria-label*="Send"]',
            'button[aria-label="Send prompt"]',
        ];

        let sendButtonEnabled = false;
        for (const selector of sendSelectors) {
            const button = document.querySelector(selector);
            if (button instanceof HTMLButtonElement && !button.disabled) {
                sendButtonEnabled = true;
                break;
            }
        }

        if (!composer) {
            return { found: false, text: '', sendButtonEnabled };
        }

        const text = 'value' in composer
            ? String(composer.value || '')
            : String(composer.innerText || composer.textContent || '');

        return {
            found: true,
            text,
            sendButtonEnabled,
        };
    });
}

async function fillComposer(chatPage, promptText) {
    const inserted = await chatPage.evaluate((text) => {
        const selectors = [
            '#prompt-textarea',
            'div[contenteditable="true"][id="prompt-textarea"]',
            'textarea[placeholder]',
            'div[contenteditable="true"][data-testid*="composer"]',
            'div[contenteditable="true"][data-testid*="prompt"]',
        ];

        let composer = null;
        for (const selector of selectors) {
            const candidate = document.querySelector(selector);
            if (candidate) {
                composer = candidate;
                break;
            }
        }

        if (!composer) {
            return false;
        }

        composer.focus();
        if ('value' in composer) {
            composer.value = '';
            composer.dispatchEvent(new Event('input', { bubbles: true }));
            composer.value = text;
            composer.dispatchEvent(new Event('input', { bubbles: true }));
            composer.dispatchEvent(new Event('change', { bubbles: true }));
            return true;
        }

        composer.textContent = '';
        composer.dispatchEvent(new InputEvent('input', {
            bubbles: true,
            inputType: 'deleteContentBackward',
            data: null,
        }));
        composer.textContent = text;
        composer.dispatchEvent(new InputEvent('input', {
            bubbles: true,
            inputType: 'insertText',
            data: text,
        }));
        return true;
    }, promptText);

    if (!inserted) {
        throw new Error('ChatGPT composer not found');
    }
}

async function submitPrompt(chatPage, promptText) {
    let lastState = null;

    for (let attempt = 1; attempt <= 3; attempt++) {
        await fillComposer(chatPage, promptText);
        await new Promise((r) => setTimeout(r, 400));

        lastState = await getComposerState(chatPage);
        if (!lastState.found) {
            throw new Error('ChatGPT composer not found');
        }
        if (!lastState.text.trim()) {
            continue;
        }

        await chatPage.keyboard.press('Enter');
        await new Promise((r) => setTimeout(r, 700));
        lastState = await getComposerState(chatPage);
        if (!lastState.text.trim()) {
            return;
        }

        const clicked = await chatPage.evaluate(() => {
            const selectors = [
                'button[data-testid="send-button"]',
                'button[aria-label*="Send"]',
                'button[aria-label="Send prompt"]',
            ];
            for (const selector of selectors) {
                const button = document.querySelector(selector);
                if (button instanceof HTMLButtonElement && !button.disabled) {
                    button.click();
                    return true;
                }
            }
            return false;
        });

        if (clicked) {
            await new Promise((r) => setTimeout(r, 700));
            lastState = await getComposerState(chatPage);
            if (!lastState.text.trim()) {
                return;
            }
        }

        await chatPage.keyboard.down('Control');
        await chatPage.keyboard.press('Enter');
        await chatPage.keyboard.up('Control');
        await new Promise((r) => setTimeout(r, 700));
        lastState = await getComposerState(chatPage);
        if (!lastState.text.trim()) {
            return;
        }
    }

    const detail = lastState
        ? `composerTextLength=${lastState.text.trim().length} sendButtonEnabled=${lastState.sendButtonEnabled}`
        : 'composerState=unavailable';
    throw new Error(`Failed to submit prompt to ChatGPT (${detail})`);
}

async function waitForAssistantResponse(chatPage, previousText = '') {
    const start = Date.now();
    let lastText = previousText;
    let stableSince = 0;

    while (Date.now() - start < RESPONSE_TIMEOUT_MS) {
        const hasStreamingUi = await chatPage.evaluate(() => {
            const selectors = [
                '[data-testid="stop-button"]',
                'button[aria-label*="Stop"]',
                'button[title*="Stop"]',
                '.result-streaming',
            ];
            return selectors.some((selector) => document.querySelector(selector));
        });

        const currentText = (await readLastAssistantMessage(chatPage)).trim();
        const hasNewText = currentText && currentText !== previousText;

        if (hasNewText && !hasStreamingUi) {
            if (currentText !== lastText) {
                lastText = currentText;
                stableSince = Date.now();
            } else if (stableSince && Date.now() - stableSince >= RESPONSE_STABLE_MS) {
                return currentText;
            }
        } else {
            stableSince = 0;
            if (currentText !== lastText) {
                lastText = currentText;
            }
        }

        await new Promise((r) => setTimeout(r, RESPONSE_POLL_MS));
    }

    throw new Error('Timed out waiting for ChatGPT to finish responding');
}

async function inspectChatGPTState(chatPage) {
    return chatPage.evaluate(() => {
        const text = document.body ? (document.body.innerText || '').toLowerCase() : '';
        const hasPrompt = Boolean(document.querySelector('#prompt-textarea'));
        const loginHints = [
            'log in',
            'sign up',
            'continue with google',
            'continue with microsoft',
            'welcome back',
        ];
        const blockedHints = [
            'verify you are human',
            'unusual activity',
            'access denied',
            'captcha',
            'checking if the site connection is secure',
        ];

        const loginRequired = !hasPrompt && loginHints.some((hint) => text.includes(hint));
        const blocked = blockedHints.some((hint) => text.includes(hint));

        return {
            hasPrompt,
            loginRequired,
            blocked,
            url: window.location.href,
            title: document.title || '',
        };
    });
}

async function ensureChatGPTReady(chatPage) {
    const state = await inspectChatGPTState(chatPage);
    if (state.blocked) {
        throw new Error(`ChatGPT page blocked or verification required at ${state.url}`);
    }
    if (state.loginRequired) {
        throw new Error(`ChatGPT login required at ${state.url}`);
    }
    if (!state.hasPrompt) {
        throw new Error(`ChatGPT prompt box not available at ${state.url}`);
    }
    return state;
}

async function findChatGPTPage(browser) {
    const pages = await browser.pages();
    for (const page of pages) {
        const url = page.url();
        if (url.includes('chatgpt.com') || url.includes('chat.openai.com')) {
            return page;
        }
    }
    return null;
}

async function recoverChatPage(browser, reason) {
    await postBridgeEvent(`Bridge recovering ChatGPT page: ${reason}`, 'info');
    appendBridgeLog({
        stage: 'chat_page_recover_start',
        reason,
    });

    for (let attempt = 1; attempt <= 5; attempt++) {
        const page = await findChatGPTPage(browser);
        if (page) {
            try {
                await page.bringToFront();
                await ensureChatGPTReady(page);
                appendBridgeLog({
                    stage: 'chat_page_recovered',
                    attempt,
                    url: page.url(),
                });
                await postBridgeEvent(`ChatGPT page recovered on attempt ${attempt}`, 'info');
                console.log(`ChatGPT recovered on attempt ${attempt}`);
                return page;
            } catch (err) {
                appendBridgeLog({
                    stage: 'chat_page_recover_retry',
                    attempt,
                    error: err.message,
                });
            }
        }
        await new Promise((r) => setTimeout(r, 2000));
    }

    throw new Error(`Unable to recover ChatGPT page after: ${reason}`);
}

async function startBridge() {
    console.log('🚀 Starting RAIG Web Bridge...');
    console.log(`📡 CONNECTING TO ENGINE AT: ${ENGINE_URL}`);
    await postBridgeEvent('Bridge process starting', 'info');
    
    try {
        const browser = await puppeteer.connect({
            browserURL: 'http://localhost:9222',
            defaultViewport: null
        });

        console.log('✅ Connected to Chrome! Scanning for ChatGPT...');
        
        let chatPage = null;
        for (let i = 0; i < 10; i++) {
            chatPage = await findChatGPTPage(browser);
            if (chatPage) break;
            if (i === 0) console.log('⏳ ChatGPT not seen yet. Waiting for page to load...');
            await new Promise(r => setTimeout(r, 1000));
        }

        if (!chatPage) {
            await postBridgeEvent('ChatGPT tab not found in Chrome', 'error');
            console.log('\n❌ ChatGPT tab STILL not found.');
            const pages = await browser.pages();
            console.log(`🔎 Current tabs: ${pages.map(p => p.url()).join(', ')}`);
            process.exit(1);
        }
        
        console.log('🎯 FOUND CHATGPT!');

        console.log('🟢 Bridge ACTIVE. Watching for signals...');
        await postBridgeEvent('Bridge connected to ChatGPT tab', 'info');

        while (true) {
            try {
                // 1. Check for pending signals
                const res = await axios.get(`${ENGINE_URL}/api/ai/pending`);
                const pending = res.data;

                if (pending && pending.length > 0) {
                    const sig = pending[0];
                    if (inFlightSignals.has(sig.id)) {
                        await new Promise(r => setTimeout(r, POLL_INTERVAL));
                        continue;
                    }

                    inFlightSignals.add(sig.id);
                    await postBridgeEvent(`Processing signal ${sig.id} ${sig.signal.action}`, 'info');
                    console.log(`📡 Signal Received: ${sig.strategyName} ${sig.signal.action}`);

                    try {
                        if (!chatPage || chatPage.isClosed()) {
                            chatPage = await recoverChatPage(browser, 'chat page missing before submit');
                        }
                        await chatPage.bringToFront();
                        const chatState = await ensureChatGPTReady(chatPage);
                        appendBridgeLog({
                            stage: 'chatgpt_ready',
                            signalId: sig.id,
                            url: chatState.url,
                            title: chatState.title,
                        });
                        await chatPage.waitForSelector('#prompt-textarea');
                        const previousReply = await readLastAssistantMessage(chatPage);
                        
                        console.log('⌨️  Pasting market data to ChatGPT...');
                        await submitPrompt(chatPage, sig.autoPrompt);

                        console.log('⏳ Waiting for ChatGPT response to finish...');
                        const rawReply = await waitForAssistantResponse(chatPage, previousReply);
                        const verdict = extractJson(rawReply);
                        const payload = {
                            id: sig.id,
                            approved: Boolean(verdict.approved),
                            action: typeof verdict.action === 'string' ? verdict.action.toUpperCase() : sig.signal.action,
                            confidence: Number(verdict.confidence || 0),
                            reason: typeof verdict.reason === 'string' ? verdict.reason : 'No reason provided',
                            rawReply
                        };

                        appendBridgeLog({
                            stage: 'parsed_verdict',
                            signalId: sig.id,
                            strategyName: sig.strategyName,
                            signalAction: sig.signal.action,
                            approved: payload.approved,
                            action: payload.action,
                            confidence: payload.confidence,
                            reason: payload.reason,
                            rawReply
                        });

                        console.log(`🧠 ChatGPT verdict: approved=${Boolean(verdict.approved)} action=${verdict.action || 'N/A'} conf=${verdict.confidence ?? 'N/A'}`);
                        await axios.post(`${ENGINE_URL}/api/ai/bridge-result`, payload);
                        await postBridgeEvent(`Submitted browser verdict for ${sig.id}`, 'info');
                        appendBridgeLog({
                            stage: 'submitted_to_engine',
                            signalId: sig.id,
                            strategyName: sig.strategyName,
                            approved: payload.approved,
                            action: payload.action,
                            confidence: payload.confidence
                        });
                    } finally {
                        inFlightSignals.delete(sig.id);
                    }
                }

                // 2. Send HEARTBEAT to engine
                try {
                    await axios.get(`${ENGINE_URL}/api/ai/bridge-heartbeat`);
                } catch (e) {}

            } catch (err) {
                if (err.message.includes('Session closed') || err.message.includes('Target closed') || err.message.includes('Page.bringToFront')) {
                    console.log('🔄 Session lost! Re-scanning for ChatGPT tab...');
                    try {
                        chatPage = await recoverChatPage(browser, err.message);
                        continue;
                    } catch (recoverErr) {
                        await postBridgeEvent(`Bridge recovery failed: ${recoverErr.message}`, 'error');
                        appendBridgeLog({
                            stage: 'bridge_error',
                            error: recoverErr.message,
                        });
                        console.error('🛑 Recovery failed:', recoverErr.message);
                    }
                } else {
                    if (err.message.includes('ChatGPT login required') || err.message.includes('ChatGPT page blocked') || err.message.includes('prompt box not available')) {
                        console.error('🛑 ChatGPT session is not ready:', err.message);
                        await postBridgeEvent(`ChatGPT session not ready: ${err.message}`, 'error');
                        try {
                            chatPage = await recoverChatPage(browser, err.message);
                            continue;
                        } catch (recoverErr) {
                            await postBridgeEvent(`Bridge recovery failed: ${recoverErr.message}`, 'error');
                            appendBridgeLog({
                                stage: 'bridge_error',
                                error: recoverErr.message,
                            });
                            console.error('🛑 Recovery failed:', recoverErr.message);
                        }
                    }
                    appendBridgeLog({
                        stage: 'bridge_error',
                        error: err.message,
                    });
                    await postBridgeEvent(`Bridge error: ${err.message}`, 'error');
                    console.error('⚠️ Bridge Loop Error:', err.message);
                }
            }
            await new Promise(r => setTimeout(r, POLL_INTERVAL));
        }

    } catch (err) {
        await postBridgeEvent(`Bridge connection error: ${err.message}`, 'error');
        console.error('❌ Connection Error:', err.message);
        console.log('TIP: Make sure Chrome is running with --remote-debugging-port=9222');
    }
}

startBridge();
