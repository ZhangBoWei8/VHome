<script setup lang="ts">
import { Bot, MessageSquarePlus, Send, Sparkles, Trash2 } from "@lucide/vue";
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from "vue";

import type { PetState } from "@/components/AgentPet.vue";
import {
  deleteAgentConversation,
  getAgentMessages,
  listAgentConversations,
  streamAgentChat,
  type AgentConversation,
  type AgentLimits,
} from "@/api";
import { usePetStore } from "@/stores/pet";
import { useSessionStore } from "@/stores/session";

type ChatMessage = {
  id: number;
  role: "user" | "assistant";
  content: string;
  failed: boolean;
};

const maxInputLength = 4000;

// The label shown while a tool runs. Tool names are internal, so each one the
// user is likely to trigger gets a plain-language equivalent.
const toolLabels: Record<string, string> = {
  pantry_list: "正在查看库存",
  pantry_search: "正在搜索物料",
  pantry_template: "正在查看物料信息",
  pantry_add: "正在入库",
  pantry_discard: "正在处理丢弃",
  meal_get_day: "正在查看当天饮食",
  meal_get_month: "正在查看饮食日历",
  meal_search_food: "正在搜索食品",
  expense_get_my_month: "正在查看我的账单",
  expense_get_summary: "正在汇总家庭账单",
  expense_list_categories: "正在查看账单分类",
  expense_query_range: "正在查询账单",
  expense_record: "正在记账",
  memo_create: "正在创建提醒",
  memo_list: "正在查看提醒",
  memory_remember: "正在记住这件事",
  memory_forget: "正在忘记这条记忆",
  memory_list: "正在回忆",
  household_list_members: "正在查看家庭成员",
};

// 空闲多久之后精灵开始搭话。
const idlePromptDelay = 30_000;

const idleSuggestions = [
  "尝试问我：冰箱里还有什么？",
  "尝试问我：这周的支出有多少？",
  "尝试问我：今天晚饭吃什么好？",
  "尝试问我：有哪些东西快过期了？",
  "想让我记住什么口味偏好，直接告诉我就行。",
];

const session = useSessionStore();
const input = ref("");
const messages = ref<ChatMessage[]>([]);
const conversations = ref<AgentConversation[]>([]);
const limits = ref<AgentLimits>({ max_rounds: 100, warn_at_rounds: 90, retention_days: 90 });
const activeConversationID = ref<number | null>(null);
const sending = ref(false);
const loadingHistory = ref(false);
const activityLabel = ref("");
const conversation = ref<HTMLElement | null>(null);

// 精灵挂在 DashboardLayout 上，这里只驱动它的状态。
const pet = usePetStore();
let idleTimer: ReturnType<typeof setTimeout> | undefined;
let happyTimer: ReturnType<typeof setTimeout> | undefined;

const activeConversation = computed(() =>
  conversations.value.find((item) => item.id === activeConversationID.value) ?? null,
);

/** 当前对话已用轮数，把正在进行的这一轮也算进去。 */
const currentRounds = computed(() => {
  const stored = activeConversation.value?.rounds ?? 0;
  const pending = Math.ceil(messages.value.length / 2);

  return Math.max(stored, pending);
});

const roundsRemaining = computed(() => limits.value.max_rounds - currentRounds.value);
const nearRoundLimit = computed(() => currentRounds.value >= limits.value.warn_at_rounds);

function setPet(state: PetState, message = "") {
  clearTimeout(happyTimer);
  pet.setState(state, message);
}

function clearIdleTimer() {
  clearTimeout(idleTimer);
  idleTimer = undefined;
}

/** 空闲一段时间后让精灵主动搭话；任何交互都会重置。 */
function scheduleIdlePrompt() {
  clearIdleTimer();
  if (sending.value) return;

  idleTimer = setTimeout(() => {
    if (sending.value) return;

    // 轮次告警优先于闲聊建议——它更要紧。
    if (nearRoundLimit.value) {
      showRoundWarning();
      return;
    }

    // 只说话，不切动作：roaming 那一行的兔子在格子里是移动的，
    // 切过去会让它看起来乱窜，也让点击位置对不上。
    const pick = idleSuggestions[Math.floor(Math.random() * idleSuggestions.length)];
    pet.say(pick ?? "", 8000);
  }, idlePromptDelay);
}

function showRoundWarning() {
  if (roundsRemaining.value > 0) {
    setPet(
      "notice",
      `这个对话快满 ${limits.value.max_rounds} 轮啦（还剩 ${roundsRemaining.value} 轮），`
        + "要不要开个新对话？满了之后最早的记录会被清掉。",
    );
    return;
  }

  setPet(
    "notice",
    `这个对话已经满 ${limits.value.max_rounds} 轮，再聊下去最早的记录会被清掉。建议开个新对话。`,
  );
}

// Negative ids for messages that exist only on the client until the turn is
// stored, so they never collide with a real row id.
let nextLocalID = -1;

const normalizedInput = computed(() => input.value.trim());
const canSend = computed(() => normalizedInput.value.length > 0 && !sending.value);

/** 用户往上翻看历史时不该被硬拽回底部，留 80px 容差判断"贴着底"。 */
function isPinnedToBottom(): boolean {
  const element = conversation.value;
  if (!element) return true;

  return element.scrollHeight - element.scrollTop - element.clientHeight < 80;
}

let scrollQueued = false;

/**
 * 滚到底部。
 *
 * 流式期间每个 token 都会调到这里，所以必须便宜：用 rAF 合并同一帧内的多次
 * 调用，并且用 auto 而不是 smooth——几百个平滑滚动动画会互相打断，看起来就是
 * 卡顿。只有切换会话这类一次性跳转才值得用 smooth。
 */
function scrollToLatest(smooth = false) {
  if (smooth) {
    void nextTick().then(() => {
      conversation.value?.scrollTo({ top: conversation.value.scrollHeight, behavior: "smooth" });
    });
    return;
  }

  if (scrollQueued || !isPinnedToBottom()) return;

  scrollQueued = true;
  requestAnimationFrame(() => {
    scrollQueued = false;
    const element = conversation.value;
    if (element) element.scrollTop = element.scrollHeight;
  });
}

async function refreshConversations() {
  try {
    const result = await listAgentConversations();
    conversations.value = result.conversations;
    limits.value = result.limits;
  } catch {
    // The thread list is a convenience; failing to load it must not block
    // the user from asking a question.
  }
}

async function selectConversation(conversationID: number) {
  if (sending.value || activeConversationID.value === conversationID) return;

  loadingHistory.value = true;
  try {
    const history = await getAgentMessages(conversationID);
    messages.value = history.map((message) => ({
      id: message.id,
      role: message.role,
      content: message.content,
      failed: false,
    }));
    activeConversationID.value = conversationID;
    scrollToLatest(true);
  } catch (cause) {
    messages.value = [{
      id: nextLocalID--,
      role: "assistant",
      content: cause instanceof Error ? cause.message : "无法加载这段对话。",
      failed: true,
    }];
  } finally {
    loadingHistory.value = false;
  }
}

function startNewConversation() {
  if (sending.value) return;

  activeConversationID.value = null;
  messages.value = [];
  input.value = "";
}

async function removeConversation(conversationID: number) {
  if (sending.value) return;

  try {
    await deleteAgentConversation(conversationID);
  } catch {
    return;
  }

  conversations.value = conversations.value.filter((item) => item.id !== conversationID);
  if (activeConversationID.value === conversationID) {
    startNewConversation();
  }
}

async function sendMessage() {
  if (!canSend.value) return;

  const question = normalizedInput.value;

  messages.value.push({ id: nextLocalID--, role: "user", content: question, failed: false });

  const reply: ChatMessage = {
    id: nextLocalID--,
    role: "assistant",
    content: "",
    failed: false,
  };
  messages.value.push(reply);

  input.value = "";
  sending.value = true;
  pet.busy = true;
  activityLabel.value = "";
  clearIdleTimer();
  setPet("thinking", "正在努力思考ing…");
  scrollToLatest(true);

  try {
    const result = await streamAgentChat(question, activeConversationID.value, {
      onConversation: (id) => {
        activeConversationID.value = id;
      },
      onTool: (name) => {
        const label = toolLabels[name] ?? "正在查询家庭数据";
        activityLabel.value = label;
        setPet("searching", `${label}…`);
      },
      onToken: (text) => {
        activityLabel.value = "";
        if (pet.state !== "speaking") setPet("speaking");
        reply.content += text;
        void scrollToLatest();
      },
    });

    reply.content = result.answer.trim() || "我暂时没有生成有效回答，请换一种说法再试试。";
    await refreshConversations();

    // 轮次告警比"答完了"更值得占用气泡。
    if (nearRoundLimit.value) {
      showRoundWarning();
    } else {
      setPet("happy");
      happyTimer = setTimeout(() => setPet("idle"), 1800);
    }
  } catch (cause) {
    reply.failed = true;
    reply.content = cause instanceof Error
      ? cause.message
      : "家庭 Agent 暂时无法回答，请稍后再试。";
    setPet("sad", "我没答上来，等会儿再试试？");
  } finally {
    sending.value = false;
    pet.busy = false;
    activityLabel.value = "";
    scrollToLatest();
    scheduleIdlePrompt();
  }
}

function handleComposerKeydown(event: KeyboardEvent) {
  if (event.key !== "Enter" || event.shiftKey || event.isComposing) return;

  event.preventDefault();
  void sendMessage();
}

// 用户一开始打字就把搭话收回去，别在输入时遮挡注意力。
watch(input, (value) => {
  if (!value) return;

  if (pet.state === "notice") setPet("idle");
  scheduleIdlePrompt();
});

// 切换会话后重新评估轮次告警。
watch(activeConversationID, () => {
  if (sending.value) return;

  if (nearRoundLimit.value) showRoundWarning();
  else setPet("idle");

  scheduleIdlePrompt();
});

onMounted(async () => {
  await refreshConversations();
  scheduleIdlePrompt();
});

onUnmounted(() => {
  clearIdleTimer();
  clearTimeout(happyTimer);
  // 交还给布局层，否则"正在思考"的气泡会跟着飘到别的页面。
  pet.busy = false;
  pet.reset();
});
</script>

<template>
  <div class="agent-page">
    <aside class="conversation-list pixel-panel">
      <header>
        <p class="panel-kicker">对话记录</p>
        <button type="button" class="new-conversation" :disabled="sending" @click="startNewConversation">
          <MessageSquarePlus :size="16" />
          <span>新对话</span>
        </button>
      </header>

      <ul v-if="conversations.length > 0">
        <li v-for="item in conversations" :key="item.id">
          <button
            type="button"
            class="conversation-item"
            :class="{ active: item.id === activeConversationID }"
            :disabled="sending"
            @click="selectConversation(item.id)"
          >
            <span class="conversation-title">{{ item.title || "新对话" }}</span>
            <small>{{ item.last_message_at }}</small>
          </button>
          <button
            type="button"
            class="delete-conversation"
            :disabled="sending"
            :aria-label="`删除对话 ${item.title || '新对话'}`"
            @click="removeConversation(item.id)"
          >
            <Trash2 :size="14" />
          </button>
        </li>
      </ul>

      <p v-else class="conversation-empty">还没有对话记录</p>
    </aside>

    <section class="agent-shell pixel-panel">
      <header class="agent-header">
        <div class="agent-identity">
          <span class="agent-avatar" aria-hidden="true"><Bot :size="27" /></span>
          <div>
            <p class="panel-kicker"><Sparkles :size="14" /> VHOME ASSISTANT</p>
            <h1>家庭 Agent</h1>
            <span><i /> 已连接家庭工具 · 会记住对话和你的长期偏好</span>
          </div>
        </div>
      </header>

      <div ref="conversation" class="conversation" aria-live="polite">
        <p v-if="loadingHistory" class="history-loading">正在加载这段对话…</p>

        <div v-else-if="messages.length === 0" class="empty-conversation">
          <span class="welcome-sprout">🌱</span>
          <h2>你好，{{ session.memberName }}</h2>
          <p>我可以帮你查找家里的物料、饮食和账单，也可以记住你的长期偏好。</p>
          <div class="prompt-example">
            <Sparkles :size="17" />
            <span>尝试问问我：“当前有哪些物料即将过期？”</span>
          </div>
        </div>

        <template v-for="message in messages" :key="message.id">
          <article v-if="message.role === 'user'" class="message-row user-message">
            <span class="message-avatar">{{ session.initials }}</span>
            <div>
              <small>你</small>
              <p>{{ message.content }}</p>
            </div>
          </article>

          <article v-else class="message-row assistant-message" :class="{ failed: message.failed }">
            <span class="message-avatar"><Bot :size="19" /></span>
            <div>
              <small>8V 家庭助手</small>
              <p v-if="message.content">{{ message.content }}</p>
              <div v-else class="thinking-indicator" aria-label="家庭 Agent 正在回答">
                <i /><i /><i />
                <span>{{ activityLabel || "正在整理家庭信息..." }}</span>
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
        <p :class="{ 'limit-near': nearRoundLimit }">
          Enter 发送 · Shift + Enter 换行 ·
          本轮 {{ currentRounds }}/{{ limits.max_rounds }} ·
          对话保存在服务器，超过 {{ limits.max_rounds }} 轮会清掉最早的记录，{{ limits.retention_days }} 天未使用的对话会被自动删除
        </p>
      </footer>
    </section>
  </div>
</template>

<style scoped>
.agent-page {
  display: grid;
  grid-template-columns: 232px minmax(0, 1fr);
  gap: 16px;
  height: calc(100vh - 162px);
  min-height: 560px;
}

.conversation-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
  min-height: 0;
  padding: 16px 12px;
  overflow-y: auto;
  background: #fffaf0;
}

.conversation-list > header {
  display: flex;
  flex-direction: column;
  gap: 9px;
}

.new-conversation {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  padding: 8px 10px;
  border: 1px solid #d9bd7e;
  border-radius: 7px;
  color: #6b4f2a;
  background: #fff3ce;
  font: inherit;
  font-size: 13px;
  cursor: pointer;
}

.new-conversation:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.conversation-list ul {
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.conversation-list li {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: center;
  gap: 2px;
}

.conversation-item {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
  padding: 8px 9px;
  border: 1px solid transparent;
  border-radius: 7px;
  color: #5d4a33;
  background: none;
  font: inherit;
  text-align: left;
  cursor: pointer;
}

.conversation-item:hover:not(:disabled),
.conversation-item.active {
  border-color: #e3cb97;
  background: #fff6dd;
}

.conversation-item:disabled {
  cursor: not-allowed;
}

.conversation-title {
  overflow: hidden;
  font-size: 13px;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.conversation-item small {
  color: #9c8a6b;
  font-size: 11px;
}

.delete-conversation {
  display: flex;
  padding: 6px;
  border: none;
  border-radius: 6px;
  color: #a98f6a;
  background: none;
  cursor: pointer;
}

.delete-conversation:hover:not(:disabled) {
  color: #a8452f;
  background: #fbe4dd;
}

.delete-conversation:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.conversation-empty,
.history-loading {
  padding: 10px 4px;
  color: #9c8a6b;
  font-size: 12px;
}

@media (max-width: 900px) {
  .agent-page {
    grid-template-columns: minmax(0, 1fr);
  }

  .conversation-list {
    max-height: 168px;
  }
}

.limit-near {
  color: #a8452f !important;
  font-weight: 600;
}

.agent-shell {
  position: relative;
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
