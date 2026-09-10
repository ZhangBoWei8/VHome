<script setup lang="ts">
import {
  BookOpenText,
  Bot,
  CalendarDays,
  ChevronLeft,
  ChevronRight,
  ClipboardList,
  CircleDollarSign,
  Home,
  LogOut,
  Lightbulb,
  Menu,
  PackageOpen,
  Settings,
  ShieldCheck,
  Snowflake,
  Sparkles,
  Users,
  Video,
  Wifi,
  X,
} from "@lucide/vue";
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";

import { useSessionStore } from "@/stores/session";
import { getDashboard, getNotifications, readMaterialReminder, type NotificationSummary, type WeatherSummary } from "@/api";

const route = useRoute();
const router = useRouter();
const session = useSessionStore();
const collapsed = ref(false);
const mobileOpen = ref(false);
const isLoggingOut = ref(false);
const notificationsOpen = ref(false);
const notifications = ref<NotificationSummary>({ pending_members: 0, material_reminders: [], memo_reminders: [] });
const weather = ref<WeatherSummary>({available:false,city:"",temperature_mean:0,weather_code:0,description:"",icon:"",date:""});
const today = new Date();
const currentMonth = new Intl.DateTimeFormat("en-US", { month: "short" }).format(today).toUpperCase();
const currentDay = today.getDate();
const notificationCount = computed(() => notifications.value.pending_members + notifications.value.material_reminders.length + notifications.value.memo_reminders.length);
const roleLabel = computed(() => ({ OWNER: "家庭所有者", ADMIN: "管理员", MEMBER: "家庭成员" }[session.memberRole ?? "MEMBER"]));
const profileAvatar = computed(() => ({
  man: "👨", woman: "👩", boy: "👦", girl: "👧", dog: "🐶",
} as Record<string, string>)[session.member?.avatar_key ?? ""] ?? session.initials);
async function loadNotifications(){try{notifications.value=await getNotifications()}catch{notifications.value={pending_members:0,material_reminders:[],memo_reminders:[]}}}
async function readReminder(itemID:number,milestone:string){await readMaterialReminder(itemID,milestone);await loadNotifications()}
function handleNotificationsChanged(){void loadNotifications()}
async function loadTopbar(){try{weather.value=(await getDashboard()).weather}catch{weather.value={available:false,city:"",temperature_mean:0,weather_code:0,description:"",icon:"",date:""}}}
function handleDashboardChanged(){void loadTopbar()}
let notificationTimer: number | undefined;
onMounted(() => {
  void loadNotifications();
  void loadTopbar();
  notificationTimer = window.setInterval(loadNotifications, 60_000);
  window.addEventListener("vhome:notifications-changed", handleNotificationsChanged);
  window.addEventListener("vhome:dashboard-changed", handleDashboardChanged);
});
onBeforeUnmount(() => {
  window.clearInterval(notificationTimer);
  window.removeEventListener("vhome:notifications-changed", handleNotificationsChanged);
  window.removeEventListener("vhome:dashboard-changed", handleDashboardChanged);
});
watch(() => route.fullPath, () => void loadNotifications());

const allNavGroups = [
  {
    label: "今日生活",
    items: [
      { label: "家庭首页", icon: Home, to: "/app" },
      { label: "物料仓库", icon: PackageOpen, to: "/app/inventory" },
      { label: "饮食日历", icon: CalendarDays, to: "/app/meals" },
      { label: "家庭账本", icon: CircleDollarSign, to: "/app/expenses" },
      { label: "家庭备忘", icon: ClipboardList, to: "/app/memos" },
    ],
  },
  {
    label: "智慧家庭",
    items: [
      { label: "家庭知识库", icon: BookOpenText, to: "/app/knowledge" },
      { label: "家庭监控", icon: Video, to: "/app/cameras" },
      { label: "智能家居", icon: Wifi, to: "/app/devices" },
      { label: "家庭 Agent", icon: Bot, to: "/app/agent" },
    ],
  },
  {
    label: "家庭管理",
    items: [
      { label: "家庭成员", icon: Users, to: "/app/members", ownerOnly: true },
      { label: "家庭设置", icon: Settings, to: "/app/settings" },
    ],
  },
];
const navGroups = computed(() => allNavGroups.map((group) => ({
  ...group,
  items: group.items.filter((item) => !("ownerOnly" in item) || session.memberRole === "OWNER"),
})));

const pageTitle = computed(() => String(route.meta.title ?? "VHome"));

function isActive(path: string) {
  return path === "/app" ? route.path === path : route.path.startsWith(path);
}

function navigate(path: string) {
  mobileOpen.value = false;
  void router.push(path);
}

async function logout() {
  if (isLoggingOut.value) return;

  isLoggingOut.value = true;

  try {
    await session.logout();
    await router.replace("/login");
  } finally {
    isLoggingOut.value = false;
  }
}
</script>

<template>
  <div class="app-shell">
    <div
      v-if="mobileOpen"
      class="sidebar-backdrop"
      aria-hidden="true"
      @click="mobileOpen = false"
    />

    <aside
      class="sidebar"
      :class="{ 'is-collapsed': collapsed, 'is-mobile-open': mobileOpen }"
      aria-label="主导航"
    >
      <div class="brand">
        <div class="brand-mark" aria-hidden="true">
          <Home :size="23" :stroke-width="2.7" />
          <span class="brand-sprout">✦</span>
        </div>
        <div v-if="!collapsed" class="brand-copy">
          <strong>VHOME</strong>
          <span>{{ session.householdName }}</span>
        </div>
        <button
          class="mobile-close icon-button"
          type="button"
          aria-label="关闭导航"
          data-tooltip-placement="right"
          @click="mobileOpen = false"
        >
          <X :size="19" />
        </button>
      </div>

      <div v-if="!collapsed" class="home-season-card">
        <span class="season-icon">🌻</span>
        <div>
          <small>盛夏 · 家庭状态</small>
          <strong><i /> 一切安好</strong>
        </div>
      </div>

      <nav class="nav-groups">
        <section v-for="group in navGroups" :key="group.label" class="nav-group">
          <p v-if="!collapsed" class="nav-label">{{ group.label }}</p>
          <button
            v-for="item in group.items"
            :key="item.to"
            class="nav-item"
            :class="{ active: isActive(item.to) }"
            type="button"
            :aria-label="collapsed ? item.label : undefined"
            data-tooltip-placement="right"
            @click="navigate(item.to)"
          >
            <component :is="item.icon" :size="20" :stroke-width="2.2" />
            <span v-if="!collapsed">{{ item.label }}</span>
            <em v-if="'badge' in item && item.badge && !collapsed">{{ item.badge }}</em>
          </button>
        </section>
      </nav>

      <div class="sidebar-footer">
        <button class="profile-mini" type="button" @click="navigate('/app/profile')">
          <span class="avatar">{{ profileAvatar }}</span>
          <span v-if="!collapsed" class="profile-copy">
            <strong>{{ session.memberName }}</strong>
            <small><ShieldCheck :size="12" /> {{ roleLabel }}</small>
          </span>
        </button>
        <button
          v-if="!collapsed"
          class="logout-button"
          type="button"
          aria-label="退出登录"
          data-tooltip-placement="top"
          :disabled="isLoggingOut"
          @click="logout"
        >
          <LogOut :size="18" />
        </button>
      </div>

      <button
        class="collapse-button"
        type="button"
        :aria-label="collapsed ? '展开导航' : '收起导航'"
        data-tooltip-placement="right"
        @click="collapsed = !collapsed"
      >
        <ChevronRight v-if="collapsed" :size="17" />
        <ChevronLeft v-else :size="17" />
      </button>
    </aside>

    <main class="main-area" :class="{ 'sidebar-collapsed': collapsed }">
      <header class="topbar">
        <div class="topbar-title">
          <button
            class="mobile-menu icon-button"
            type="button"
            aria-label="打开导航"
            data-tooltip-placement="right"
            @click="mobileOpen = true"
          >
            <Menu :size="21" />
          </button>
          <div>
            <p>{{ pageTitle }}</p>
            <span>把家里的每件小事，安放得刚刚好。</span>
          </div>
        </div>

        <div class="topbar-actions">
          <div class="weather-pill">
            <span>{{ weather.available ? weather.icon : "🌤️" }}</span>
            <div>
              <strong>{{ weather.available ? `${weather.temperature_mean}°C` : "--°C" }}</strong>
              <small>{{ weather.available ? `${weather.city} · ${weather.description}` : weather.city ? `${weather.city} · 天气暂不可用` : "请设置家庭所在地" }}</small>
            </div>
          </div>
          <div class="date-pill">
            <small>{{ currentMonth }}</small>
            <strong>{{ currentDay }}</strong>
          </div>
          <button class="icon-button notification-button" type="button" aria-label="查看家庭提醒" data-tooltip-placement="left" @click="notificationsOpen=!notificationsOpen">
            <Lightbulb :size="20" />
            <i v-if="notificationCount" />
          </button>
          <div v-if="notificationsOpen" class="notification-popover pixel-panel">
            <header><strong>家庭提醒</strong><span>{{notificationCount}} 条</span></header>
            <button v-if="notifications.pending_members" class="notice-row" @click="navigate('/app/members');notificationsOpen=false">
              <span>💡</span><p><strong>{{notifications.pending_members}} 位成员等待审批</strong><small>点击前往家庭成员页面</small></p>
            </button>
            <button v-for="n in notifications.material_reminders" :key="`${n.item_id}-${n.milestone}`" class="notice-row" @click="readReminder(n.item_id,n.milestone)">
              <span>⏳</span><p><strong>{{n.item_name}} · {{n.message}}</strong><small>{{n.remaining_days===0?'今天到期':`剩余 ${n.remaining_days} 天`}}，点击标记已读</small></p>
            </button>
            <button v-for="memo in notifications.memo_reminders" :key="`memo-${memo.id}`" class="notice-row" @click="navigate(`/app/memos?memo=${memo.id}`);notificationsOpen=false">
              <span>📝</span><p><strong>{{memo.title}}</strong><small>{{new Date(memo.remind_at).toLocaleString('zh-CN',{month:'numeric',day:'numeric',hour:'2-digit',minute:'2-digit',hour12:false})}} · 点击查看备忘</small></p>
            </button>
            <p v-if="!notificationCount" class="notice-empty">暂时没有新的提醒 🌿</p>
          </div>
          <button class="top-avatar" type="button" aria-label="编辑个人资料" data-tooltip-placement="left" @click="navigate('/app/profile')">
            {{ profileAvatar }}
          </button>
        </div>
      </header>

      <div class="content-wrap">
        <RouterView />
      </div>
    </main>

    <div class="ambient-decoration ambient-snowflake"><Snowflake /></div>
    <div class="ambient-decoration ambient-sparkle"><Sparkles /></div>
  </div>
</template>
