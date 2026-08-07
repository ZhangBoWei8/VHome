<script setup lang="ts">
import {
  ArrowRight,
  CalendarClock,
  ChevronRight,
  CircleDollarSign,
  PackageCheck,
  Plus,
  Snowflake,
  ThermometerSun,
  Users,
} from "@lucide/vue";
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import { useRouter } from "vue-router";

import {
  getDashboard,
  type DashboardSummary,
  type PresenceStatus,
} from "@/api";
import pantryIcons from "@/assets/pantry-icons.png";
import { useSessionStore } from "@/stores/session";

const router = useRouter();
const session = useSessionStore();
const dashboard = ref<DashboardSummary | null>(null);
const loading = ref(true);
const error = ref("");

const iconOrder = [
  "milk", "egg", "leafy-vegetable", "potato", "scallion",
  "pork", "beef", "chicken", "fish", "fruit",
  "chips", "canned-food", "rice", "bread", "seasoning",
  "generic-food", "fridge", "pantry", "generic-storage", "drink",
];

const unitLabels: Record<string, string> = {
  G: "克", PIECE: "个", PACK: "包", BOX: "盒", BOTTLE: "瓶", CAN: "罐",
};

const statusOptions: Array<{ value: PresenceStatus | ""; label: string; emoji: string }> = [
  { value: "", label: "未设置", emoji: "🌿" },
  { value: "HOME", label: "在家", emoji: "🏡" },
  { value: "SCHOOL", label: "在校", emoji: "🏫" },
  { value: "WORKING", label: "工作ing", emoji: "💻" },
  { value: "OUT", label: "外出", emoji: "🚲" },
  { value: "NAPPING", label: "午睡", emoji: "😴" },
  { value: "RESTING", label: "休息ing", emoji: "☕" },
  { value: "SICK", label: "生病", emoji: "🤒" },
  { value: "STUDYING", label: "学习ing", emoji: "📖" },
];

function iconStyle(key: string) {
  const index = Math.max(0, iconOrder.indexOf(key));
  return {
    backgroundImage: `url(${pantryIcons})`,
    backgroundSize: "500% 400%",
    backgroundPosition: `${(index % 5) * 25}% ${Math.floor(index / 5) * (100 / 3)}%`,
  };
}

const now = new Date();
const weekday = new Intl.DateTimeFormat("en-US", { weekday: "long" })
  .format(now)
  .toUpperCase();
const season = computed(() => {
  const month = now.getMonth() + 1;
  if (month >= 3 && month <= 5) return "春日";
  if (month === 6) return "初夏";
  if (month === 7 || month === 8) return "盛夏";
  if (month === 9) return "初秋";
  if (month === 10 || month === 11) return "深秋";
  return "冬日";
});
const greeting = computed(() => {
  const hour = new Date().getHours();
  if (hour < 6) return "夜深了";
  if (hour < 11) return "早上好";
  if (hour < 14) return "中午好";
  if (hour < 18) return "下午好";
  return "晚上好";
});

const welcomeText = computed(() => {
  const count = dashboard.value?.attention_count ?? 0;
  if (count === 0) return "今天家里一切安好，暂时没有需要留意的临期物料。";
  return `今天家里一切安好。有 ${count} 件物料需要你留意。`;
});

function statusMeta(status: PresenceStatus | "") {
  return statusOptions.find((item) => item.value === status) ?? {
    value: "" as const,
    label: "未设置",
    emoji: "🌿",
  };
}

function avatarDisplay(key: string, displayName: string) {
  return ({
    man: "👨", woman: "👩", boy: "👦", girl: "👧", dog: "🐶",
  } as Record<string, string>)[key] ?? displayName.slice(0, 1) ?? "?";
}

function expiryText(days: number) {
  if (days === 0) return "今天到期";
  if (days === 1) return "明天到期";
  return `剩 ${days} 天`;
}

function relativeTime(value: string) {
  const diff = Date.now() - new Date(value).getTime();
  const minutes = Math.max(0, Math.floor(diff / 60_000));
  if (minutes < 1) return "刚刚";
  if (minutes < 60) return `${minutes} 分钟前`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours} 小时前`;
  return new Intl.DateTimeFormat("zh-CN", { month: "short", day: "numeric" }).format(new Date(value));
}

async function loadDashboard() {
  error.value = "";
  try {
    dashboard.value = await getDashboard();
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : "读取家庭看板失败";
  } finally {
    loading.value = false;
  }
}

function handleDashboardChanged() {
  void loadDashboard();
}

onMounted(() => {
  void loadDashboard();
  window.addEventListener("vhome:dashboard-changed", handleDashboardChanged);
  window.addEventListener("vhome:notifications-changed", handleDashboardChanged);
});
onBeforeUnmount(() => {
  window.removeEventListener("vhome:dashboard-changed", handleDashboardChanged);
  window.removeEventListener("vhome:notifications-changed", handleDashboardChanged);
});
</script>

<template>
  <div class="dashboard-page page-stack">
    <p v-if="error" class="form-message error">{{ error }}</p>

    <section class="welcome-banner">
      <div class="welcome-copy">
        <p class="eyebrow">{{ weekday }} · {{ season }}第 {{ now.getDate() }} 天</p>
        <h1>{{ greeting }}，{{ session.memberName }} <span>🌻</span></h1>
        <p>{{ loading ? "正在整理今天的家庭信息…" : welcomeText }}</p>
        <div class="welcome-actions">
          <button class="primary-pixel-button compact" @click="router.push('/app/inventory')">
            <Plus :size="17" /> 添加库存
          </button>
          <button class="text-action" @click="router.push('/app/inventory')">
            查看仓库 <ArrowRight :size="16" />
          </button>
        </div>
      </div>
      <div class="welcome-illustration" aria-hidden="true">
        <span class="mini-sun">☀</span>
        <div class="mini-house"><i class="mini-roof" /><i class="mini-body">⌂</i></div>
        <span class="mini-tree">♣</span>
        <span class="mini-grass">˄ ˄ ˄ ˄ ˄ ˄ ˄</span>
      </div>
    </section>

    <section class="metric-grid">
      <article class="metric-card peach">
        <div class="metric-icon"><PackageCheck /></div>
        <div>
          <p>家庭物料</p>
          <strong>{{ dashboard?.inventory_count ?? 0 }} <small>件</small></strong>
          <span>当前可用库存</span>
        </div>
      </article>
      <article class="metric-card butter">
        <div class="metric-icon"><CalendarClock /></div>
        <div>
          <p>临期提醒</p>
          <strong>{{ dashboard?.attention_count ?? 0 }} <small>件</small></strong>
          <span class="attention">
            {{ dashboard?.due_today_count ? `其中 ${dashboard.due_today_count} 件今天到期` : "今天没有物料到期" }}
          </span>
        </div>
      </article>
      <article class="metric-card mint development-card">
        <div class="metric-icon"><ThermometerSun /></div>
        <div><p>室内环境</p><strong>开发中</strong><span>等待智能设备接入</span></div>
      </article>
      <article class="metric-card sky development-card">
        <div class="metric-icon"><CircleDollarSign /></div>
        <div><p>本月家庭开销</p><strong>开发中</strong><span>等待家庭账本功能</span></div>
      </article>
    </section>

    <section class="dashboard-grid">
      <article class="pixel-panel expiring-panel">
        <header class="panel-header">
          <div><p class="panel-kicker">PANTRY WATCH</p><h2>需要留意的物料</h2></div>
          <button type="button" @click="router.push('/app/inventory')">查看全部 <ChevronRight :size="16" /></button>
        </header>
        <div v-if="dashboard?.pantry_watch.length" class="expiring-list">
          <button
            v-for="item in dashboard.pantry_watch"
            :key="item.id"
            class="expiring-row"
            type="button"
            @click="router.push('/app/inventory')"
          >
            <span class="food-avatar dashboard-food-icon">
              <i class="material-pixel-icon" :style="iconStyle(item.icon_key)" />
            </span>
            <span class="food-copy">
              <strong>{{ item.name }}</strong>
              <small>{{ item.location_name }} · {{ item.quantity }} {{ unitLabels[item.unit] ?? item.unit }}</small>
            </span>
            <span class="expiry-chip" :class="item.remaining_days <= 1 ? 'danger' : item.remaining_days <= 3 ? 'warning' : 'safe'">
              {{ expiryText(item.remaining_days) }}
            </span>
            <ChevronRight :size="17" />
          </button>
        </div>
        <div v-else class="dashboard-empty"><span>🌱</span><p>暂时没有需要留意的临期物料</p></div>
      </article>

      <article class="pixel-panel storage-panel">
        <header class="panel-header">
          <div><p class="panel-kicker">STORAGE MAP</p><h2>储存空间</h2></div>
          <Snowflake :size="20" class="panel-symbol" />
        </header>
        <div class="storage-map">
          <template v-for="storage in dashboard?.storage ?? []" :key="storage.storage_type">
            <div class="storage-visual" :class="storage.storage_type === 'COLD' ? 'fridge' : 'cabinet'">
              <span class="storage-emoji">{{ storage.storage_type === "COLD" ? "❄️" : "🗄️" }}</span>
              <div>
                <strong>{{ storage.storage_type === "COLD" ? "低温存储" : "常温存储" }}</strong>
                <small>{{ storage.location_count }} 个位置 · {{ storage.item_count }} 件物料</small>
              </div>
              <b>{{ storage.percentage }}%</b>
            </div>
            <div class="storage-progress"><i :style="{ width: `${storage.percentage}%` }" /></div>
          </template>
          <div v-if="!dashboard?.storage.length" class="dashboard-empty compact"><p>还没有储存空间数据</p></div>
        </div>
      </article>

      <article class="pixel-panel activity-panel">
        <header class="panel-header"><div><p class="panel-kicker">RECENT ACTIVITY</p><h2>最近动态</h2></div></header>
        <div v-if="dashboard?.activities.length" class="activity-list">
          <div v-for="activity in dashboard.activities" :key="`${activity.type}-${activity.happened_at}`" class="activity-row">
            <span class="activity-icon" :class="activity.type === 'INVENTORY_ADDED' ? 'yellow' : activity.type === 'INVENTORY_DISCARDED' ? 'brown' : 'blue'">
              {{ activity.type === "INVENTORY_ADDED" ? "📦" : activity.type === "INVENTORY_DISCARDED" ? "🧺" : "🏡" }}
            </span>
            <div><strong>{{ activity.text }}</strong><small>{{ relativeTime(activity.happened_at) }}</small></div>
          </div>
        </div>
        <div v-else class="dashboard-empty compact"><span>🍃</span><p>还没有家庭动态</p></div>
      </article>

      <article class="pixel-panel family-panel">
        <header class="panel-header">
          <div><p class="panel-kicker">FAMILY AT A GLANCE</p><h2>家人此刻</h2></div>
          <Users :size="20" class="panel-symbol" />
        </header>
        <div class="member-list dashboard-member-list">
          <div v-for="member in dashboard?.members ?? []" :key="member.id" class="member-item">
            <span>{{ avatarDisplay(member.avatar_key, member.display_name) }}</span>
            <strong>{{ member.display_name }}</strong>
            <small>{{ statusMeta(member.presence_status).emoji }} {{ statusMeta(member.presence_status).label }}</small>
          </div>
        </div>
      </article>
    </section>
  </div>
</template>
