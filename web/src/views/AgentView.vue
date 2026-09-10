<script setup lang="ts">
import { Bot, Info, Send, Sparkles } from "@lucide/vue";
import { computed, nextTick, ref } from "vue";

import { chatWithAgent } from "@/api";
import { useSessionStore } from "@/stores/session";

type ChatRound = {
  id: number;
  question: string;
  answer: string;
  failed: boolean;
};

const maxVisibleRounds = 10;
const maxInputLength = 4000;

const session = useSessionStore();
const input = ref("");
const rounds = ref<ChatRound[]>([]);
const sending = ref(false);
const conversation = ref<HTMLElement | null>(null);
let nextRoundID = 1;

const normalizedInput = computed(() => input.value.trim());
const canSend = computed(() => normalizedInput.value.length > 0 && !sending.value);

async function scrollToLatest() {
  await nextTick();
  conversation.value?.scrollTo({
    top: conversation.value.scrollHeight,
    behavior: "smooth",
  });
}

async function sendMessage() {
  if (!canSend.value) return;

  const question = normalizedInput.value;
  const round: ChatRound = {
    id: nextRoundID++,
    question,
    answer: "",
    failed: false,
  };

  rounds.value.push(round);
  if (rounds.value.length > maxVisibleRounds) {
    rounds.value.splice(0, rounds.value.length - maxVisibleRounds);
  }

  input.value = "";
  sending.value = true;
  await scrollToLatest();

  try {
    const result = await chatWithAgent(question);
    round.answer = result.answer.trim() || "我暂时没有生成有效回答，请换一种说法再试试。";
  } catch (cause) {
    round.failed = true;
    round.answer = cause instanceof Error
      ? cause.message
      : "家庭 Agent 暂时无法回答，请稍后再试。";
  } finally {
    sending.value = false;
    await scrollToLatest();
  }
}

function handleComposerKeydown(event: KeyboardEvent) {
  if (event.key !== "Enter" || event.shiftKey || event.isComposing) return;

  event.preventDefault();
  void sendMessage();
}
</script>

<template>
  <div class="agent-page">
    <section class="agent-shell pixel-panel">
      <header class="agent-header">
        <div class="agent-identity">
          <span class="agent-avatar" aria-hidden="true"><Bot :size="27" /></span>
          <div>
            <p class="panel-kicker"><Sparkles :size="14" /> VHOME ASSISTANT</p>
            <h1>家庭 Agent</h1>
            <span><i /> 测试版本 · 已连接家庭工具</span>
          </div>
        </div>
        <div class="memory-notice">
          <Info :size="18" />
          <p>
            <strong>当前为临时对话</strong>
            本页仅展示最近 10 轮，刷新或离开后会清空。Agent 暂时不会跨轮保留上下文，后续将加入记忆库、历史记录与更多能力。
          </p>
        </div>
      </header>

      <div ref="conversation" class="conversation" aria-live="polite">
        <div v-if="rounds.length === 0" class="empty-conversation">
          <span class="welcome-sprout">🌱</span>
          <h2>你好，{{ session.memberName }}</h2>
          <p>我可以帮你查找家里的物料、物料基础信息和个人账单。</p>
          <div class="prompt-example">
            <Sparkles :size="17" />
            <span>尝试问问我：“当前有哪些物料即将过期？”</span>
          </div>
        </div>

        <template v-for="round in rounds" :key="round.id">
          <article class="message-row user-message">
            <span class="message-avatar">{{ session.initials }}</span>
            <div>
              <small>你</small>
              <p>{{ round.question }}</p>
            </div>
          </article>

          <article class="message-row assistant-message" :class="{ failed: round.failed }">
            <span class="message-avatar"><Bot :size="19" /></span>
            <div>
              <small>8V 家庭助手</small>
              <p v-if="round.answer">{{ round.answer }}</p>
              <div v-else class="thinking-indicator" aria-label="家庭 Agent 正在回答">
                <i /><i /><i />
                <span>正在整理家庭信息...</span>
              </div>
            </div>
          </article>
        </template>
      </div>

      <footer class="composer-area">
        <div class="composer" :class="{ 'is-busy': sending }">
          <label class="sr-only" for="agent-input">向家庭 Agent 提问</label>
          <textarea
            id="agent-input"
            v-model="input"
            :maxlength="maxInputLength"
            :disabled="sending"
            rows="1"
            placeholder="输入问题，例如：鸡蛋什么时候过期？"
            @keydown="handleComposerKeydown"
          />
          <button
            type="button"
            :disabled="!canSend"
            :aria-label="sending ? '正在发送' : '发送消息'"
            @click="sendMessage"
          >
            <Send :size="19" />
            <span>{{ sending ? "回答中" : "发送" }}</span>
          </button>
        </div>
        <p>Enter 发送 · Shift + Enter 换行 · 内容仅用于本次页面会话</p>
      </footer>
    </section>
  </div>
</template>

<style scoped>
.agent-page {
  height: calc(100vh - 162px);
  min-height: 560px;
}

.agent-shell {
  display: grid;
  grid-template-rows: auto minmax(0, 1fr) auto;
  height: 100%;
  min-height: 0;
  overflow: hidden;
  background:
    linear-gradient(rgba(255, 255, 255, 0.32), rgba(255, 255, 255, 0)) padding-box,
    #fffaf0;
}

.agent-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 24px;
  padding: 20px 24px;
  border-bottom: 2px solid #ead6a5;
  background: rgba(255, 247, 222, 0.82);
}

.agent-identity {
  display: flex;
  align-items: center;
  gap: 14px;
  flex: 0 0 auto;
}

.agent-avatar {
  display: grid;
  place-items: center;
  width: 52px;
  height: 52px;
  border: 2px solid var(--wood-700);
  border-radius: 8px;
  color: #fffbea;
  background: var(--grass-500);
  box-shadow: inset 0 0 0 3px #91b464, 0 4px 0 rgba(83, 50, 37, 0.22);
}

.agent-identity h1 {
  margin-top: 2px;
  color: var(--wood-800);
  font-family: "Pixelify Sans", sans-serif;
  font-size: 27px;
}

.agent-identity > div > span {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 3px;
  color: #806750;
  font-size: 12px;
}

.agent-identity > div > span i {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--grass-400);
  box-shadow: 0 0 0 3px rgba(126, 166, 83, 0.16);
}

.memory-notice {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  max-width: 570px;
  padding: 11px 13px;
  border: 1px solid #d9bd7e;
  border-radius: 7px;
  color: #765d46;
  background: #fff3ce;
  font-size: 12px;
  line-height: 1.55;
}

.memory-notice svg {
  flex: 0 0 auto;
  margin-top: 2px;
  color: var(--yellow);
}

.memory-notice strong {
  display: block;
  margin-bottom: 1px;
  color: var(--wood-700);
}

.conversation {
  min-height: 0;
  overflow-y: auto;
  padding: 28px clamp(22px, 5vw, 72px);
  scrollbar-color: #b98a58 #f6e9c7;
  scrollbar-width: thin;
}

.conversation::-webkit-scrollbar {
  width: 10px;
}

.conversation::-webkit-scrollbar-track {
  border-left: 1px solid #ecd8aa;
  background: #f6e9c7;
}

.conversation::-webkit-scrollbar-thumb {
  border: 2px solid #f6e9c7;
  border-radius: 5px;
  background: #b98a58;
}

.empty-conversation {
  display: grid;
  justify-items: center;
  max-width: 610px;
  margin: clamp(38px, 9vh, 94px) auto 0;
  text-align: center;
}

.welcome-sprout {
  display: grid;
  place-items: center;
  width: 68px;
  height: 68px;
  border: 2px solid #a97a4d;
  border-radius: 50%;
  background: #f5e5b8;
  box-shadow: inset 0 0 0 4px #fff6dc, 0 5px 0 rgba(83, 50, 37, 0.15);
  font-size: 34px;
}

.empty-conversation h2 {
  margin-top: 18px;
  color: var(--wood-800);
  font-family: "Pixelify Sans", sans-serif;
  font-size: 28px;
}

.empty-conversation > p {
  margin-top: 8px;
  color: #806b56;
  font-size: 14px;
}

.prompt-example {
  display: flex;
  align-items: center;
  gap: 9px;
  margin-top: 24px;
  padding: 12px 16px;
  border: 1px dashed #bf9a60;
  border-radius: 7px;
  color: var(--grass-600);
  background: #fff6dc;
  font-size: 13px;
  font-weight: 700;
}

.message-row {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  max-width: 820px;
  margin: 0 auto 24px;
}

.message-avatar {
  display: grid;
  place-items: center;
  flex: 0 0 auto;
  width: 38px;
  height: 38px;
  border: 2px solid var(--wood-600);
  border-radius: 7px;
  color: #fffbed;
  background: var(--wood-400);
  box-shadow: 0 3px 0 rgba(83, 50, 37, 0.18);
  font-family: "Pixelify Sans", sans-serif;
  font-weight: 700;
}

.message-row > div {
  min-width: 0;
}

.message-row small {
  display: block;
  margin: 1px 0 6px;
  color: #8e745c;
  font-size: 11px;
  font-weight: 800;
}

.message-row p {
  width: fit-content;
  max-width: 100%;
  padding: 12px 15px;
  border: 1px solid #d8bd85;
  border-radius: 4px 10px 10px 10px;
  color: var(--wood-800);
  background: #fffdf7;
  box-shadow: 0 3px 0 rgba(83, 50, 37, 0.09);
  font-size: 14px;
  line-height: 1.7;
  overflow-wrap: anywhere;
  white-space: pre-wrap;
}

.user-message {
  flex-direction: row-reverse;
}

.user-message > div {
  display: grid;
  justify-items: end;
}

.user-message .message-avatar {
  color: var(--wood-800);
  background: #e8b66f;
}

.user-message p {
  border-radius: 10px 4px 10px 10px;
  color: #fffdf4;
  background: var(--grass-500);
}

.assistant-message.failed p {
  border-color: #cf8e7d;
  color: #8c3f35;
  background: #fff0e9;
}

.thinking-indicator {
  display: flex;
  align-items: center;
  gap: 5px;
  min-height: 46px;
  padding: 12px 15px;
  border: 1px solid #d8bd85;
  border-radius: 4px 10px 10px 10px;
  background: #fffdf7;
}

.thinking-indicator i {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--grass-400);
  animation: agent-bounce 1.1s infinite ease-in-out;
}

.thinking-indicator i:nth-child(2) { animation-delay: 120ms; }
.thinking-indicator i:nth-child(3) { animation-delay: 240ms; }

.thinking-indicator span {
  margin-left: 5px;
  color: #8d755d;
  font-size: 12px;
}

.composer-area {
  padding: 16px clamp(20px, 5vw, 68px) 13px;
  border-top: 2px solid #ead6a5;
  background: #fff7df;
}

.composer {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: end;
  gap: 12px;
  max-width: 900px;
  margin: 0 auto;
  padding: 8px 8px 8px 15px;
  border: 2px solid #c8a467;
  border-radius: 9px;
  background: #fffdf7;
  box-shadow: inset 0 2px 0 rgba(98, 62, 36, 0.04), 0 4px 0 rgba(83, 50, 37, 0.12);
  transition: border-color 150ms ease, box-shadow 150ms ease;
}

.composer:focus-within {
  border-color: var(--grass-500);
  box-shadow: 0 0 0 3px rgba(99, 135, 68, 0.13), 0 4px 0 rgba(83, 50, 37, 0.12);
}

.composer textarea {
  width: 100%;
  min-height: 42px;
  max-height: 118px;
  padding: 10px 0 7px;
  resize: vertical;
  border: 0;
  outline: 0;
  color: var(--wood-800);
  background: transparent;
  font: inherit;
  font-size: 14px;
  line-height: 1.55;
}

.composer textarea::placeholder {
  color: #a18a73;
}

.composer button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 7px;
  min-width: 94px;
  min-height: 42px;
  padding: 0 15px;
  border: 2px solid var(--wood-700);
  border-radius: 6px;
  color: #fffdf0;
  background: var(--grass-500);
  box-shadow: inset 0 0 0 2px #8bad5f, 0 3px 0 var(--wood-700);
  cursor: pointer;
  font-weight: 800;
}

.composer button:not(:disabled):hover {
  background: var(--grass-600);
  transform: translateY(-1px);
}

.composer button:disabled {
  cursor: not-allowed;
  filter: saturate(0.55);
  opacity: 0.5;
}

.composer-area > p {
  margin-top: 9px;
  color: #947a61;
  text-align: center;
  font-size: 11px;
}

@keyframes agent-bounce {
  0%, 70%, 100% { transform: translateY(0); opacity: 0.55; }
  35% { transform: translateY(-5px); opacity: 1; }
}

@media (max-width: 900px) {
  .agent-header {
    align-items: stretch;
    flex-direction: column;
    gap: 13px;
    padding: 17px;
  }

  .memory-notice {
    max-width: none;
  }

  .conversation {
    padding: 22px 16px;
  }

  .composer-area {
    padding: 13px;
  }
}

@media (max-width: 720px) {
  .agent-page {
    height: calc(100dvh - 127px);
  }
}

@media (max-width: 560px) {
  .agent-page {
    height: calc(100dvh - 127px);
    min-height: 500px;
  }

  .agent-header {
    padding: 13px;
  }

  .agent-avatar {
    width: 44px;
    height: 44px;
  }

  .agent-identity h1 {
    font-size: 23px;
  }

  .memory-notice {
    font-size: 11px;
  }

  .conversation {
    padding: 18px 12px;
  }

  .empty-conversation {
    margin-top: 24px;
  }

  .prompt-example {
    align-items: flex-start;
    text-align: left;
  }

  .message-row {
    gap: 8px;
  }

  .message-avatar {
    width: 34px;
    height: 34px;
  }

  .composer {
    gap: 7px;
    padding-left: 11px;
  }

  .composer button {
    min-width: 44px;
    width: 44px;
    padding: 0;
  }

  .composer button span {
    display: none;
  }
}
</style>
