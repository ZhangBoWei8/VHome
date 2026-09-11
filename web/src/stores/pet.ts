import { defineStore } from "pinia";
import { computed, ref } from "vue";

import type { PetState } from "@/components/AgentPet.vue";

/**
 * 精灵的全局状态。
 *
 * 精灵挂在 DashboardLayout 上，登录后每个页面都在；但驱动它的是 AgentView
 * （对话过程中的思考/查询/说话）。两者不在同一棵组件树里，所以状态放 store。
 */

const positionStorageKey = "vhome.pet.position";

/** 每个页面的闲聊内容。agent 页的提示由 AgentView 自己接管，这里不管。 */
const chatterByRoute: Record<string, string[]> = {
  dashboard: [
    "今天过得怎么样？",
    "有什么要我帮忙的，去「家庭 Agent」找我就行。",
    "记得看看有没有快过期的东西～",
  ],
  inventory: [
    "想知道什么快过期了？来问我。",
    "新买的东西记得入库哦。",
  ],
  meals: [
    "今天想吃点什么？我可以帮你想。",
    "把忌口告诉我，我会一直记着。",
  ],
  expenses: [
    "这个月花了多少，我可以帮你算。",
    "随口说一句就能记账，不用自己填表。",
  ],
  memos: [
    "要我提醒你什么吗？",
    "定好的提醒我会按时发出去。",
  ],
  members: ["家里谁在家，我随时能查。"],
  profile: ["换个头像试试？"],
  settings: ["通知设置好了，提醒才发得出去。"],
};

const fallbackChatter = ["快来「家庭 Agent」找我聊天！"];

type Position = { x: number; y: number };

export const usePetStore = defineStore("pet", () => {
  const state = ref<PetState>("idle");
  const message = ref("");
  const position = ref<Position | null>(loadPosition());

  // AgentView 接管期间（正在回答），布局层的闲聊不要插嘴。
  const busy = ref(false);

  let messageTimer: ReturnType<typeof setTimeout> | undefined;

  function loadPosition(): Position | null {
    try {
      const raw = localStorage.getItem(positionStorageKey);
      if (!raw) return null;

      const parsed = JSON.parse(raw) as Position;
      if (typeof parsed?.x !== "number" || typeof parsed?.y !== "number") return null;

      return parsed;
    } catch {
      // 隐私模式或禁用了站点数据：用默认位置，不是错误。
      return null;
    }
  }

  function savePosition(next: Position) {
    position.value = next;
    try {
      localStorage.setItem(positionStorageKey, JSON.stringify(next));
    } catch {
      // 存不下就算了，位置只是个便利。
    }
  }

  /**
   * 回到默认角落。
   *
   * 进入 agent 页时调用：对话页需要精灵待在可预期的位置，不然它可能停在
   * 消息中间挡视线。其他页面随便拖，拖到哪存到哪。
   */
  function resetPosition() {
    position.value = null;
    try {
      localStorage.removeItem(positionStorageKey);
    } catch {
      // 同上，失败无所谓。
    }
  }

  function clearMessageTimer() {
    clearTimeout(messageTimer);
    messageTimer = undefined;
  }

  /** 设置状态；ttl 毫秒后自动回到待机。 */
  function setState(next: PetState, text = "", ttl = 0) {
    clearMessageTimer();
    state.value = next;
    message.value = text;

    if (ttl > 0) {
      messageTimer = setTimeout(() => {
        state.value = "idle";
        message.value = "";
      }, ttl);
    }
  }

  /** 只说一句话，不改动作。 */
  function say(text: string, ttl = 6000) {
    clearMessageTimer();
    message.value = text;

    if (ttl > 0) {
      messageTimer = setTimeout(() => { message.value = ""; }, ttl);
    }
  }

  /** 立刻闭嘴，但保持当前动作。拖动精灵时用。 */
  function hush() {
    clearMessageTimer();
    message.value = "";
  }

  function reset() {
    clearMessageTimer();
    state.value = "idle";
    message.value = "";
  }

  /** 按当前路由挑一句闲聊。 */
  function chatterFor(routeName: string): string {
    const pool = chatterByRoute[routeName] ?? fallbackChatter;

    // 索引访问在 noUncheckedIndexedAccess 下是 string | undefined，
    // 空数组也确实可能出现（以后有人把某一页的闲聊清空）。
    return pool[Math.floor(Math.random() * pool.length)] ?? fallbackChatter[0]!;
  }

  const hasMessage = computed(() => message.value.length > 0);

  return {
    state, message, position, busy, hasMessage,
    setState, say, hush, reset, savePosition, resetPosition, chatterFor,
  };
});
